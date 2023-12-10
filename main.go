package main

import (
	"fmt"
	"log"
	"strconv"
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/golang-jwt/jwt/v5"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

type UserCredential struct {
	Username string `json:"username,omitempty"`
	Password string `json:"password,omitempty"`
}

type UserPassport struct {
	Id       int  `json:"id"`
	Admin    bool `json:"admin"`
	Graduate bool `json:"graduate"`
	Employer bool `json:"employer"`
}

type User struct {
	UserPassport
	UserCredential
}

func (u *User) hideSensitiveData() {
	u.UserCredential.Password = ""
}

// Job properties inspired by : https://www.indeed.com/viewjob?jk=5d43c4aa2edf6f41&tk=1hh1n8q22jkuc800&from=serp&vjs=3
type Job struct {
	Id             int    `json:"id"`
	Title          string `json:"title"`
	Role           string `json:"role"`            // Role & Year of experience (mandatory)
	YearExperience int    `json:"year_experience"` // Not necessary since similar to role experience
	// Status         bool     `json:"status"`
	// Description    string   `json:"description"`
	// Skills         []string `json:"skills"` // Skills & Year of experience (optional)
	// Salary         []int    `json:"salary"` // Min & Max
	// Company        string   `json:"company"`
	// City           string   `json:"city"`
}

func (j Job) isValid() error {
	var err error = nil

	if j.Title == "" || j.Role == "" {
		err = fmt.Errorf("Job title and role are mandatory when creating new Job")
		return fiber.ErrBadGateway
	}

	if j.YearExperience < 0 {
		err = fmt.Errorf("Minimum Year of Experience can't go below 0 for creating a new Job")
		return err
	}

	return err
}

type JobApplication struct {
	Id         int  `json:"id"`
	GraduateId int  `json:"graduate_id"`
	JobId      int  `json:"job_id"`
	Graduate   User `gorm:"foreignKey:GraduateId"`
	Job        Job  `gorm:"foreignKey:JobId"`
}

func (j JobApplication) isValid() error {
	var err error = nil

	// Part 1: Check if the application already exist
	result := []JobApplication{}
	DB.Where("job_id = ? AND graduate_id = ?", j.JobId, j.GraduateId).Find(&result)

	if len(result) > 0 {
		err = fmt.Errorf("This graduate has Already applied to this Job")
		return err
	}

	// Part 2: Check if the job and graduate exist to avoid broken references
	jobs := []Job{}
	graduates := []User{}

	DB.Where("id = ?", j.JobId).Find(&jobs)
	DB.Where("id = ?", j.GraduateId).Find(&graduates)

	if len(jobs) == 0 || len(graduates) == 0 {
		err = fmt.Errorf("Graduate or Job not found in the system")
		return err
	}

	return err
}

type Friendship struct {
	Id     int  `json:"id"`
	FromId int  `json:"from"`
	ToId   int  `json:"to"`
	From   User `gorm:"foreignKey:FromId"`
	To     User `gorm:"foreignKey:ToId"`
}

func (f *Friendship) isValid() error {
	var err error = nil

	if f.FromId == f.ToId {
		err = fmt.Errorf("You can't add yourself as a friend")
		return err
	}

	friendship := []Friendship{}
	DB.Where("from_id = ? AND to_id = ?", f.FromId, f.ToId).
		Or("from_id = ? AND to_id = ?", f.ToId, f.FromId).
		Find(&friendship)

	if len(friendship) > 0 {
		err = fmt.Errorf("Friendship already exists")
		return err
	}

	users := []User{}
	DB.Where("id IN ?", []string{strconv.Itoa(f.FromId), strconv.Itoa(f.ToId)}).
		Find(&users)

	if len(users) != 2 {
		err = fmt.Errorf("User not found in the system")
		return err
	}

	// FIXME: I don't like this one (this is a side effect)
	// It should be its own function
	if f.FromId == users[0].Id {
		f.From = users[0]
		f.To = users[1]
	} else {
		f.From = users[1]
		f.To = users[0]
	}

	return err
}

func (f *Friendship) hideSensitiveData() {
	f.From.hideSensitiveData()
	f.To.hideSensitiveData()
}

func main() {

	// 1 -- Database Definition
	// os.Remove("./jobs.db")

	db, err := gorm.Open(sqlite.Open("./jobs.db"), &gorm.Config{})
	if err != nil {
		log.Fatal("Upcoming error")
		panic("failed to connect to the database")
	}

	DB = db
	db.AutoMigrate(&Job{})
	db.AutoMigrate(&User{})
	db.AutoMigrate(&JobApplication{})
	db.AutoMigrate(&Friendship{})

	// 2 -- Launching the server
	app := fiber.New()
	setupRoute(app)

	port := ":2200"
	app.Listen(port)
}

var secret_key string = "hello"
var DB *gorm.DB

func setupRoute(app *fiber.App) {
	app.Get("/", func(c *fiber.Ctx) error {
		return c.Status(fiber.StatusOK).JSON(fiber.Map{
			"hello": "world !",
		})
	})

	// ==================================================
	// ==================================================
	//                        API
	// ==================================================
	// ==================================================

	api := app.Group("/api/v1")

	api.Post("/registration", func(c *fiber.Ctx) error {
		user := &User{}

		if err := c.BodyParser(user); err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"message": err.Error(),
			})
		}

		// Check that the user doesn't already exist before creating it
		existingUser := &[]User{}
		DB.Limit(1).Find(existingUser, "username = ?", user.Username)

		if len(*existingUser) > 0 {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"message": "User with username '" + user.Username + "' already exists",
			})
		}

		DB.Create(user)

		return c.Status(fiber.StatusOK).JSON(fiber.Map{
			"data": user,
		})
	})

	api.Post("/login", func(c *fiber.Ctx) error {
		// Fetch User data
		userCredential := UserCredential{}

		if err := c.BodyParser(&userCredential); err != nil {
			c.Status(fiber.StatusBadGateway).JSON(fiber.Map{
				"message": err.Error(),
			})
		}

		// Check User in DB
		existingUsers := []User{}
		DB.Where("username = ? AND password = ?", userCredential.Username, userCredential.Password).Limit(1).Find(&existingUsers)

		if len(existingUsers) == 0 {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"message": "Invalid Username or Password",
			})
		}

		// If user found, send token back to client
		userPassport := UserPassport{
			Id:       existingUsers[0].Id,
			Admin:    existingUsers[0].Admin,
			Graduate: existingUsers[0].Graduate,
			Employer: existingUsers[0].Employer,
		}

		claims := jwt.MapClaims{
			"passport": userPassport,
			// "exp":      time.Now().Add(time.Minute * 5).Unix(),
		}

		key := []byte(secret_key)
		token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
		token_string, _ := token.SignedString(key)

		return c.Status(fiber.StatusOK).JSON(fiber.Map{
			"token": token_string,
		})
	})

	// ==================================================
	// ==================================================
	//                   Middleware
	// ==================================================
	// ==================================================

	api.Use(jwtMiddlewareProtect)

	api.Get("/jobs", graduateOnlyMiddleware, func(c *fiber.Ctx) error {
		availableJobs := []Job{}

		DB.Find(&availableJobs)

		return c.Status(fiber.StatusOK).JSON(fiber.Map{
			"jobs": availableJobs,
		})
	})

	api.Post("/jobs", employerOnlyMiddleware, func(c *fiber.Ctx) error {
		job := Job{}

		if err := c.BodyParser(&job); err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"message": err.Error(),
			})
		}

		if err := job.isValid(); err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"message": err.Error(),
			})
		}

		DB.Create(&job)

		return c.Status(fiber.StatusOK).JSON(fiber.Map{
			"job": job,
		})
	})

	// FIXME: "GET" methods send user password as well
	// Remove it from the JSON response
	api.Post("/application", graduateOnlyMiddleware, func(c *fiber.Ctx) error {
		application := JobApplication{}

		if err := c.BodyParser(&application); err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"message": err.Error(),
			})
		}

		if err := application.isValid(); err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"message": err.Error(),
			})
		}

		DB.Create(&application)

		return c.Status(fiber.StatusOK).JSON(fiber.Map{
			"job_application": application,
		})
	})

	// TODO: Add additional "Middleware" to protect this route
	api.Get("/application", func(c *fiber.Ctx) error {
		applications := []JobApplication{}

		DB.Preload("Job").Preload("Graduate").Find(&applications)

		return c.Status(fiber.StatusOK).JSON(fiber.Map{
			"job_applications": applications,
		})
	})

	// TODO: Add additional "Middleware" to protect this route
	api.Get("/application/:job_id<int>/:graduate_id<int>", func(c *fiber.Ctx) error {
		var jobId string = c.Params("job_id")
		var graduateId string = c.Params("graduate_id")

		applications := []JobApplication{}

		DB.Preload("Graduate").Preload("Job").
			Where("job_id = ? AND graduate_id = ?", jobId, graduateId).
			Find(&applications)

		if len(applications) == 0 {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
				"message": "Application to this Job not found for this Graduate",
			})
		}

		if len(applications) > 1 {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"message": "Internal Server Error -- Multiple Applications found to the same Job for this Graduate",
			})
		}

		return c.Status(fiber.StatusOK).JSON(fiber.Map{
			"job_application": applications[0],
		})
	})

	api.Get("/application/job/:job_id", func(c *fiber.Ctx) error {
		var jobId string = c.Params("job_id")

		applications := []JobApplication{}
		DB.Preload("Graduate").Preload("Job").
			Where("job_id = ?", jobId).
			Find(&applications)

		return c.Status(fiber.StatusOK).JSON(fiber.Map{
			"job_applications": applications,
		})
	})

	api.Get("/application/graduate/:graduate_id", func(c *fiber.Ctx) error {
		var graduateId string = c.Params("graduate_id")

		applications := []JobApplication{}
		DB.Preload("Graduate").Preload("Job").
			Where("graduate_id = ?", graduateId).
			Find(&applications)

		return c.Status(fiber.StatusOK).JSON(fiber.Map{
			"job_applications": applications,
		})
	})

	api.Get("/user/graduate", func(c *fiber.Ctx) error {
		graduates := []User{}

		DB.Where("graduate = true").Find(&graduates)

		return c.Status(fiber.StatusOK).JSON(fiber.Map{
			"graduates": graduates,
		})
	})

	api.Get("/user/employer", func(c *fiber.Ctx) error {
		employers := []User{}

		DB.Where("employer = true").Find(&employers)

		return c.Status(fiber.StatusOK).JSON(fiber.Map{
			"employers": employers,
		})
	})

	api.Get("/friends", func(c *fiber.Ctx) error {
		friends := []Friendship{}

		DB.Preload("From").Preload("To").Find(&friends)

		hideSensitiveFriendshipData(&friends)

		return c.Status(fiber.StatusOK).JSON(fiber.Map{
			"friends": friends,
		})
	})

	api.Post("/friends", func(c *fiber.Ctx) error {
		friendship := Friendship{}

		if err := c.BodyParser(&friendship); err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"message": err.Error(),
			})
		}

		if err := friendship.isValid(); err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"message": err.Error(),
			})
		}

		DB.Create(&friendship)

		friendship.hideSensitiveData()

		return c.Status(fiber.StatusOK).JSON(fiber.Map{
			"friends": friendship,
		})
	})

	api.Get("/friends/:my_id", func(c *fiber.Ctx) error {
		id := c.Params("my_id")

		friends := []Friendship{}
		DB.Where("from_id = ? OR to_id = ?", id, id).
			Preload("From").Preload("To").
			Find(&friends)

		hideSensitiveFriendshipData(&friends)

		return c.Status(fiber.StatusOK).JSON(fiber.Map{
			"friends": friends,
		})
	})

	api.Get("/friends/:my_id/:friend_id", func(c *fiber.Ctx) error {
		myId := c.Params("my_id")
		friendId := c.Params("friend_id")

		friends := []Friendship{}
		DB.Where("(from_id = ? AND to_id = ?) OR (from_id = ? AND to_id = ?)", myId, friendId, friendId, myId).
			Preload("From").Preload("To").
			Find(&friends)

		if len(friends) == 0 {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
				"message": "Friendship not found",
			})
		}

		hideSensitiveFriendshipData(&friends)

		return c.Status(fiber.StatusOK).JSON(fiber.Map{
			"friend": friends[0],
		})
	})
}

func hideSensitiveFriendshipData(friends *[]Friendship) {
	for key, _ := range *friends {
		friend := &(*friends)[key]

		friend.hideSensitiveData()
	}
}

func hideSensitiveUserData(users *[]User) {
	for key, _ := range *users {
		user := &(*users)[key]

		(*user).hideSensitiveData()
	}
}

func graduateEmployerOnlyMiddleware(c *fiber.Ctx) error {
	var passport UserPassport = getUserPassportFromMiddlewareContext(c)

	if passport.Admin == true {
		return c.Next()
	}

	if passport.Graduate == true || passport.Employer == true {
		return c.Next()
	}

	return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
		"message": "Unauthorized, only Graduate and Employer can access",
	})
}

func graduateOnlyMiddleware(c *fiber.Ctx) error {
	var passport UserPassport = getUserPassportFromMiddlewareContext(c)

	if passport.Admin == true {
		return c.Next()
	}

	if passport.Graduate == true {
		return c.Next()
	}

	return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
		"message": "Unauthorized, only Graduate can access",
	})
}

func employerOnlyMiddleware(c *fiber.Ctx) error {
	var passport UserPassport = getUserPassportFromMiddlewareContext(c)

	if passport.Admin == true {
		return c.Next()
	}

	if passport.Employer == true {
		return c.Next()
	}

	return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
		"message": "Unauthorized, only Employer can access",
	})
}

func adminOnlyMiddleware(c *fiber.Ctx) error {
	var passport UserPassport = getUserPassportFromMiddlewareContext(c)

	if passport.Admin == true {
		return c.Next()
	}

	return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
		"message": "Unauthorized, only Admin can access",
	})
}

func getUserPassportFromMiddlewareContext(c *fiber.Ctx) UserPassport {
	passport := c.Locals("user_passport")
	fmt.Printf("\n UserPassport = %v \n", passport)
	userPassport := passport.(UserPassport)

	return userPassport
}

// Middleware that check if the token is valid (that the user is registered in the site)
func jwtMiddlewareProtect(c *fiber.Ctx) error {
	token_string := extractTokenFromAuthHeader(c)
	fmt.Printf("Token String = %v \n", token_string)

	type CustomClaims struct {
		jwt.RegisteredClaims
		Passport UserPassport `json:"passport"`
	}

	// claims := jwt.MapClaims{}
	claims := &CustomClaims{}
	token, err := jwt.ParseWithClaims(token_string, claims, func(token *jwt.Token) (interface{}, error) {
		return []byte(secret_key), nil
	})

	if err != nil || !token.Valid {
		return err
	}

	c.Locals("user_passport", claims.Passport)

	return c.Next()
}

func extractTokenFromAuthHeader(c *fiber.Ctx) string {
	fmt.Println(c.GetReqHeaders())

	headers := c.GetReqHeaders()
	str := headers["Authorization"][0]

	val_str := strings.Split(str, "BEARER")
	token_string := strings.Trim(val_str[1], " ")

	return token_string
}
