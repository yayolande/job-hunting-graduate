package main

import (
	"database/sql"
	"fmt"
	"log"
	"strconv"
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/golang-jwt/jwt/v5"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	_ "github.com/mattn/go-sqlite3"
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
	gormDB.Where("job_id = ? AND graduate_id = ?", j.JobId, j.GraduateId).Find(&result)

	if len(result) > 0 {
		err = fmt.Errorf("This graduate has Already applied to this Job")
		return err
	}

	// Part 2: Check if the job and graduate exist to avoid broken references
	jobs := []Job{}
	graduates := []User{}

	gormDB.Where("id = ?", j.JobId).Find(&jobs)
	gormDB.Where("id = ?", j.GraduateId).Find(&graduates)

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
	gormDB.Where("from_id = ? AND to_id = ?", f.FromId, f.ToId).
		Or("from_id = ? AND to_id = ?", f.ToId, f.FromId).
		Find(&friendship)

	if len(friendship) > 0 {
		err = fmt.Errorf("Friendship already exists")
		return err
	}

	users := []User{}
	gormDB.Where("id IN ?", []string{strconv.Itoa(f.FromId), strconv.Itoa(f.ToId)}).
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

type Message struct {
	Id         int    `json:"id"`
	SenderId   int    `json:"sender_id"`
	ReceiverId int    `json:"receiver_id"`
	Message    string `json:"message"`
	Sender     User   `gorm:"ForeignKey:SenderId" json:"-"`
	Receiver   User   `gorm:"ForeignKey:ReceiverId" json:"-"`
}

func (m Message) isValid() error {
	var err error = nil

	users := []User{}
	gormDB.Where("id IN ?", []string{strconv.Itoa(m.SenderId), strconv.Itoa(m.ReceiverId)}).
		Find(&users)

	if len(users) != 2 && len(users) != 1 {
		err = fmt.Errorf("User not found in the system")
		return err
	}

	if len(m.Message) == 0 {
		err = fmt.Errorf("Message can't be empty")
		return err
	}

	return err
}

type JobSkill struct {
	Id   int    `json:"id"`
	Name string `json:"name"`
}

type JobRole struct {
	Id   int    `json:"id"`
	Name string `json:"name"`
}

type CurriculumVitae struct {
	Id         int          `json:"id"`
	Gpa        float32      `json:"gpa"`
	Yoe        float32      `json:"yoe"`
	GraduateId int          `json:"graduate_id"`
	JobRoleId  int          `json:"job_role_id"`
	Graduate   User         `json:"user"`
	JobRole    JobRole      `json:"job_role"`
	Tree       []SkillsTree `json:"tree"`
}

type SkillsTree struct {
	Id         int      `json:"id"`
	JobSkillId int      `json:"job_skill_id"`
	GraduateId int      `json:"graduate_id"`
	JobSkill   JobSkill `json:"job_skill"`
	Graduate   User     `json:"graduate"`
}

func main() {

	// 1 -- Database Definition
	// os.Remove("./jobs.gormDb")

	gormDb, err := gorm.Open(sqlite.Open("./jobs.db"), &gorm.Config{})
	if err != nil {
		log.Fatal("Upcoming error")
		panic("failed to connect to the database")
	}

	gormDB = gormDb
	gormDb.AutoMigrate(&Job{})
	gormDb.AutoMigrate(&User{})
	gormDb.AutoMigrate(&JobApplication{})
	gormDb.AutoMigrate(&Friendship{})
	gormDb.AutoMigrate(&Message{})

	db, err := sql.Open("sqlite3", "./jobs.db")
	if err != nil {
		fmt.Println("Unable to open Database. Error : ", err.Error())
		return
	}

	DB = db
	databaseMigration(db)

	// skill := JobSkill{Name: "Jest Testing"}
	// role := JobRole{Name: "Nurse"}

	// saveJobSkillsToDB(db, skill)
	// saveJobRoleToDB(db, role)

	// saveSkillsTreeToDB(db, SkillsTree{JobSkillId: 30, GraduateId: 2})
	// saveSkillsTreeToDB(db, SkillsTree{JobSkillId: 29, GraduateId: 2})

	cv, err := getGraduateCurriculumViateFromDB(db, 3)
	cvs, err := getAllGraduateCurriculumViateFromDB(db)

	fmt.Println("CV fetching from DB: ")
	fmt.Println(cv)
	fmt.Println("CVS fetching from DB: ")
	fmt.Println(cvs)

	// return

	// 2 -- Launching the server
	app := fiber.New()
	setupRoute(app)

	port := ":2200"
	app.Listen(port)
}

var secret_key string = "hello"
var gormDB *gorm.DB
var DB *sql.DB

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
		gormDB.Limit(1).Find(existingUser, "username = ?", user.Username)

		if len(*existingUser) > 0 {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"message": "User with username '" + user.Username + "' already exists",
			})
		}

		gormDB.Create(user)

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

		// Check User in gormDB
		existingUsers := []User{}
		gormDB.Where("username = ? AND password = ?", userCredential.Username, userCredential.Password).Limit(1).Find(&existingUsers)

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

		gormDB.Find(&availableJobs)

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

		gormDB.Create(&job)

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

		gormDB.Create(&application)

		return c.Status(fiber.StatusOK).JSON(fiber.Map{
			"job_application": application,
		})
	})

	// TODO: Add additional "Middleware" to protect this route
	api.Get("/application", func(c *fiber.Ctx) error {
		applications := []JobApplication{}

		gormDB.Preload("Job").Preload("Graduate").Find(&applications)

		return c.Status(fiber.StatusOK).JSON(fiber.Map{
			"job_applications": applications,
		})
	})

	// TODO: Add additional "Middleware" to protect this route
	api.Get("/application/:job_id<int>/:graduate_id<int>", func(c *fiber.Ctx) error {
		var jobId string = c.Params("job_id")
		var graduateId string = c.Params("graduate_id")

		applications := []JobApplication{}

		gormDB.Preload("Graduate").Preload("Job").
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
		gormDB.Preload("Graduate").Preload("Job").
			Where("job_id = ?", jobId).
			Find(&applications)

		return c.Status(fiber.StatusOK).JSON(fiber.Map{
			"job_applications": applications,
		})
	})

	api.Get("/application/graduate/:graduate_id", func(c *fiber.Ctx) error {
		var graduateId string = c.Params("graduate_id")

		applications := []JobApplication{}
		gormDB.Preload("Graduate").Preload("Job").
			Where("graduate_id = ?", graduateId).
			Find(&applications)

		return c.Status(fiber.StatusOK).JSON(fiber.Map{
			"job_applications": applications,
		})
	})

	api.Get("/user/graduate", func(c *fiber.Ctx) error {
		graduates := []User{}

		gormDB.Where("graduate = true").Find(&graduates)

		return c.Status(fiber.StatusOK).JSON(fiber.Map{
			"graduates": graduates,
		})
	})

	api.Get("/user/employer", func(c *fiber.Ctx) error {
		employers := []User{}

		gormDB.Where("employer = true").Find(&employers)

		return c.Status(fiber.StatusOK).JSON(fiber.Map{
			"employers": employers,
		})
	})

	api.Get("/friends", func(c *fiber.Ctx) error {
		friends := []Friendship{}

		gormDB.Preload("From").Preload("To").Find(&friends)

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

		gormDB.Create(&friendship)

		friendship.hideSensitiveData()

		return c.Status(fiber.StatusOK).JSON(fiber.Map{
			"friends": friendship,
		})
	})

	api.Get("/friends/:my_id", func(c *fiber.Ctx) error {
		id := c.Params("my_id")

		friends := []Friendship{}
		gormDB.Where("from_id = ? OR to_id = ?", id, id).
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
		gormDB.Where("(from_id = ? AND to_id = ?) OR (from_id = ? AND to_id = ?)", myId, friendId, friendId, myId).
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

	api.Get("/messages", func(c *fiber.Ctx) error {
		messages := []Message{}

		gormDB.Find(&messages)

		return c.Status(fiber.StatusOK).JSON(fiber.Map{
			"messages": messages,
		})
	})

	api.Post("/messages", func(c *fiber.Ctx) error {
		message := Message{}

		if err := c.BodyParser(&message); err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"message": err.Error(),
			})
		}

		if err := message.isValid(); err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"message": err.Error(),
			})
		}

		gormDB.Create(&message)

		return c.Status(fiber.StatusOK).JSON(fiber.Map{
			"message": message,
		})
	})

	api.Get("/messages/:sender_id<int>/:receiver_id<int>", func(c *fiber.Ctx) error {
		senderId := c.Params("sender_id")
		receiverId := c.Params("receiver_id")

		messages := []Message{}
		gormDB.Where("sender_id = ? AND receiver_id = ? OR sender_id = ? AND receiver_id = ?", senderId, receiverId, receiverId, senderId).
			Find(&messages)

		return c.Status(fiber.StatusOK).JSON(fiber.Map{
			"messages": messages,
		})
	})

	api.Get("/messages/lasts/:user_id<int>", func(c *fiber.Ctx) error {
		user_id := c.Params("user_id")

		messages := []Message{}
		gormDB.Where("sender_id = ? OR receiver_id = ?", user_id, user_id).
			Find(&messages)

		// Finding out the last message for each conversion between two users
		counter := 0
		lastMessages := []Message{}

		lastMessages = append(lastMessages, messages[0])

		for _, message := range messages {
			for key, lastMsg := range lastMessages {
				if (lastMsg.SenderId == message.SenderId && lastMsg.ReceiverId == message.ReceiverId) ||
					(lastMsg.SenderId == message.ReceiverId && lastMsg.ReceiverId == message.SenderId) {
					lastMessages[key] = message
					break
				}

				counter = key
			}

			if counter == len(lastMessages)-1 {
				lastMessages = append(lastMessages, message)
			}
		}

		messages = lastMessages

		return c.Status(fiber.StatusOK).JSON(fiber.Map{
			"messages": messages,
		})
	})

	api.Get("/cv", func(c *fiber.Ctx) error {
		cvs, err := getAllGraduateCurriculumViateFromDB(DB)

		if err != nil {
			log.Println("[API route ./cv] error while fetching all graduate cv. ", err.Error())
			return c.Status(fiber.StatusNoContent).JSON(fiber.Map{
				"error": err,
			})
		}

		return c.Status(fiber.StatusOK).JSON(fiber.Map{
			"cv": cvs,
		})
	})

	api.Post("/cv", func(c *fiber.Ctx) error {
		cv := CurriculumVitae{}

		if err := c.BodyParser(&cv); err != nil {
			message := fmt.Sprintln("[POST /cv] Error while parsing input data. Error: ", err.Error())
			log.Print(message)
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"message": message,
			})
		}

		err := saveCurriculumVitae(DB, cv)
		if err != nil {
			message := "Unable to save the data to database"
			log.Println(message, " ---> ", err.Error())
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"message": message,
			})
		}

		return c.Status(fiber.StatusNotImplemented).JSON(fiber.Map{
			"cv": cv,
		})
	})

	api.Get("/cv/simple", func(c *fiber.Ctx) error {
		cvs, err := getCurriculumVitae(DB)
		if err != nil {
			fmt.Println("[GET /cv/simple] unable to get data from db. Error : ", err.Error())
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"message": "Unable to get curriculum_vitae from database",
			})
		}

		return c.Status(fiber.StatusOK).JSON(fiber.Map{
			"cvs": cvs,
		})
	})

	api.Get("/cv/skills", func(c *fiber.Ctx) error {
		return c.SendStatus(fiber.StatusNotImplemented)
	})

	api.Post("/cv/skills", func(c *fiber.Ctx) error {
		skill := SkillsTree{}

		if err := c.BodyParser(&skill); err != nil {
			fmt.Println("[POST /cv/skills] Error :", err.Error())
			return c.SendStatus(fiber.StatusBadRequest)
		}

		err := saveSkillsTreeToDB(DB, skill)
		if err != nil {
			fmt.Println("[POST /cv/skills] Error :", err.Error())
			return c.SendStatus(fiber.StatusInternalServerError)
		}

		return c.Status(fiber.StatusOK).JSON(fiber.Map{
			"skill": skill,
		})
	})

	api.Get("/skills", func(c *fiber.Ctx) error {
		skills, err := getJobSkillsFromDB(DB)
		if err != nil {
			fmt.Println("[Error while fetching job_skills from db] ", err.Error())
			return c.SendStatus(fiber.StatusInternalServerError)
		}

		return c.Status(fiber.StatusOK).JSON(fiber.Map{
			"skills": skills,
		})
	})

	api.Post("/skills", func(c *fiber.Ctx) error {
		skill := JobSkill{}

		if err := c.BodyParser(&skill); err != nil {
			fmt.Println("[POST /skills] request parsing error : ", err.Error())
			return c.SendStatus(fiber.StatusBadRequest)
		}

		err := saveJobSkillsToDB(DB, skill)
		if err != nil {
			return c.SendStatus(fiber.StatusInternalServerError)
		}

		return c.Status(fiber.StatusOK).JSON(fiber.Map{
			"skill": skill,
		})
	})

	api.Get("/job_roles", func(c *fiber.Ctx) error {
		roles, err := getJobRoleFromDB(DB)

		if err != nil {
			fmt.Println("[GET /job_roles] ", err.Error())
			return c.SendStatus(fiber.StatusInternalServerError)
		}

		return c.Status(fiber.StatusOK).JSON(fiber.Map{
			"job_roles": roles,
		})
	})

	api.Post("/job_roles", func(c *fiber.Ctx) error {
		role := JobRole{}

		if err := c.BodyParser(&role); err != nil {
			fmt.Println("[POST /job_roles] ", err.Error())
			return c.SendStatus(fiber.StatusBadRequest)
		}

		err := saveJobRoleToDB(DB, role)
		if err != nil {
			fmt.Println("[POST /job_roles] ", err.Error())
			return c.SendStatus(fiber.StatusInternalServerError)
		}

		return c.Status(fiber.StatusOK).JSON(fiber.Map{
			"job_role": role,
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
	if len(headers["Authorization"]) == 0 {
		fmt.Println("[Warning] Authorization headers not found")
		return ""
	}
	str := headers["Authorization"][0]

	if !strings.Contains(str, "BEARER") {
		fmt.Println("[Error] Malformated Authorization Headers, 'BEARER' separator not present")
		return ""
	}

	val_str := strings.Split(str, "BEARER")
	token_string := strings.Trim(val_str[1], " ")

	return token_string
}

func databaseMigration(db *sql.DB) (err error) {
	var sqlStmt string = ""

	sqlStmt = `
    CREATE TABLE IF NOT EXISTS job_roles (
      id integer primary key autoincrement,
      name varchar(100)
    );
  `

	err = databaseExec(db, sqlStmt)

	sqlStmt = `
    CREATE TABLE IF NOT EXISTS job_skills (
      id integer primary key autoincrement,
      name varchar(100)
    );
  `
	err = databaseExec(db, sqlStmt)

	sqlStmt = `
    CREATE TABLE IF NOT EXISTS skills_tree (
      id integer primary key autoincrement,
      job_skill_id int,
      graduate_id int,
      FOREIGN KEY (graduate_id) REFERENCES users (id),
      FOREIGN KEY (job_skill_id) REFERENCES job_skills (id)
    );
  `

	err = databaseExec(db, sqlStmt)

	sqlStmt = `
    CREATE TABLE IF NOT EXISTS curriculum_vitae (
      id integer primary key autoincrement,
      gpa int CHECK (gpa >= 0),
      job_role_id int,
      yoe float,
      graduate_id int,
      FOREIGN KEY (job_role_id) REFERENCES job_roles (id),
      FOREIGN KEY (graduate_id) REFERENCES users (id)
    );
  `

	err = databaseExec(db, sqlStmt)

	return err
}

func databaseExec(db *sql.DB, sqlStmt string) error {
	_, err := db.Exec(sqlStmt)
	if err != nil {
		log.Print("[DB error] ", err.Error(), " \n --> For SQL query : ", sqlStmt)
		return err
	}

	return nil
}

func saveJobSkillsToDB(db *sql.DB, skill JobSkill) (err error) {
	var sqlStmt string = `
    INSERT INTO job_skills (name) VALUES (?);
  `
	_, err = db.Exec(sqlStmt, skill.Name)
	if err != nil {
		log.Println(err.Error(), " ---> ", sqlStmt)
	}

	return err
}

func saveJobRoleToDB(db *sql.DB, role JobRole) (err error) {
	var sqlStmt string = `
    INSERT INTO job_roles (name) VALUES (?);
  `
	_, err = db.Exec(sqlStmt, role.Name)
	if err != nil {
		log.Println(err.Error(), " ---> ", sqlStmt)
	}

	return err
}

func getJobRoleFromDB(db *sql.DB) ([]JobRole, error) {
	var sqlStmt string = `
    SELECT * FROM job_roles;
  `

	rows, err := db.Query(sqlStmt)
	if err != nil {
		log.Println(err.Error(), " ---> ", sqlStmt)
		return nil, err
	}
	defer rows.Close()

	roles := []JobRole{}
	role := JobRole{}

	for rows.Next() {
		err = rows.Scan(&role.Id, &role.Name)
		if err != nil {
			log.Println(err.Error(), " ---> ", sqlStmt)
			continue
		}

		roles = append(roles, role)
	}
	if err = rows.Err(); err != nil {
		log.Println(err.Error(), " ---> ", sqlStmt)
		return nil, err
	}

	return roles, nil
}

func saveCurriculumVitae(db *sql.DB, cv CurriculumVitae) (err error) {
	var sqlStmt string = `
    INSERT INTO curriculum_vitae (gpa, job_role_id, yoe, graduate_id) 
    VALUES (?, ?, ?, ?);
  `
	_, err = db.Exec(sqlStmt, cv.Gpa, cv.JobRoleId, cv.Yoe, cv.GraduateId)
	if err != nil {
		log.Println(err.Error(), " ---> ", sqlStmt)
	}

	return err
}

func saveSkillsTreeToDB(db *sql.DB, tree SkillsTree) (err error) {
	var sqlStmt string = `
    INSERT INTO skills_tree (job_skill_id, graduate_id) 
    VALUES (?, ?);
  `
	_, err = db.Exec(sqlStmt, tree.JobSkillId, tree.GraduateId)
	if err != nil {
		log.Println(err.Error(), " ---> ", sqlStmt)
	}

	return err
}

func getJobSkillsFromDB(db *sql.DB) ([]JobSkill, error) {
	var sqlStmt string = `
    SELECT * FROM job_skills;
  `
	rows, err := db.Query(sqlStmt)
	if err != nil {
		log.Println(err.Error(), " ---> ", sqlStmt)
		return nil, err
	}
	defer rows.Close()

	skills := []JobSkill{}
	skill := JobSkill{}

	for rows.Next() {
		err = rows.Scan(&skill.Id, &skill.Name)
		if err != nil {
			log.Println(err.Error(), " ---> ", sqlStmt)
			continue
		}

		skills = append(skills, skill)
	}

	err = rows.Err()
	if err != nil {
		log.Println(err.Error(), " ---> ", sqlStmt)
		return nil, err
	}

	return skills, nil
}

func getGraduateCurriculumViateFromDB(db *sql.DB, graduateId int) (*CurriculumVitae, error) {
	var sqlStmt string = `
    select c.id, c.gpa, c.yoe, r.id AS role_id, r.name AS role_name, t.* 
    from curriculum_vitae c 
    inner join users u on c.graduate_id = u.id 
    inner join job_roles r on c.job_role_id = r.id
    inner join (select u.id AS user_id, u.username AS user_name, s.id AS skill_id, s.name AS skill_name, t.id AS tree_id from skills_tree t inner join job_skills s on t.job_skill_id = s.id inner join users u on t.graduate_id = u.id) t on c.graduate_id = t.user_id
    WHERE t.user_id = ?;
  `

	rows, err := db.Query(sqlStmt, graduateId)
	// rows, err := db.Query(sqlStmt)
	if err != nil {
		log.Println(err.Error(), " ---> ", sqlStmt)

		return nil, err
	}

	cv := CurriculumVitae{}
	tree := SkillsTree{}

	cv.Tree = []SkillsTree{}

	for rows.Next() {
		tree = SkillsTree{}
		// fmt.Println("rows: ", rows)

		err = rows.Scan(
			&cv.Id, &cv.Gpa, &cv.Yoe,
			&cv.JobRole.Id, &cv.JobRole.Name, &cv.Graduate.Id, &cv.Graduate.Username,
			&tree.JobSkill.Id, &tree.JobSkill.Name, &tree.Id,
		)

		if err != nil {
			log.Println(err.Error(), " ---> rows.Scan() error ---> ", sqlStmt)
			continue
		}

		cv.GraduateId = cv.Graduate.Id
		cv.JobRoleId = cv.JobRole.Id
		tree.Graduate = cv.Graduate
		tree.GraduateId = tree.Graduate.Id
		tree.JobSkillId = tree.JobSkill.Id

		cv.Tree = append(cv.Tree, tree)
	}

	err = rows.Err()
	if err != nil {
		log.Println(err.Error(), " ---> ", sqlStmt)

		return nil, err
	}

	return &cv, nil
}

func getCurriculumVitae(db *sql.DB) ([]CurriculumVitae, error) {
	var sqlStmt = `
    SELECT * FROM curriculum_vitae;
  `

	rows, err := db.Query(sqlStmt)
	if err != nil {
		log.Println(err.Error(), " ---> ", sqlStmt)
		return nil, err
	}
	defer rows.Close()

	cvs := []CurriculumVitae{}
	cv := CurriculumVitae{}

	for rows.Next() {
		err = rows.Scan(&cv.Id, &cv.Gpa, &cv.JobRoleId, &cv.Yoe, &cv.GraduateId)
		if err != nil {
			log.Println("[Rows scan error] get CurriculumVitae error : ", err)
		}

		cvs = append(cvs, cv)
	}

	if err = rows.Err(); err != nil {
		log.Println(err.Error(), " ---> ", sqlStmt)
		return nil, err
	}

	return cvs, nil
}

func getAllGraduateCurriculumViateFromDB(db *sql.DB) ([]CurriculumVitae, error) {
	var sqlStmt string = `
    select c.id, c.gpa, c.yoe, r.id AS role_id, r.name AS role_name, t.* 
    from curriculum_vitae c 
    inner join users u on c.graduate_id = u.id 
    inner join job_roles r on c.job_role_id = r.id
    inner join (
      select u.id AS user_id, u.username AS user_name, s.id AS skill_id, s.name AS skill_name, t.id AS tree_id 
      from skills_tree t 
      inner join job_skills s on t.job_skill_id = s.id 
      inner join users u on t.graduate_id = u.id
      where t.graduate_id = u.id
    ) t on c.graduate_id = t.user_id;
  `

	rows, err := db.Query(sqlStmt)
	if err != nil {
		log.Println(err.Error(), " ---> ", sqlStmt)

		return nil, err
	}

	var prevCv CurriculumVitae
	cvs := []CurriculumVitae{}
	cv := CurriculumVitae{}
	tree := SkillsTree{}

	cv.Tree = []SkillsTree{}

	const ID_EMPTY int = -1
	graduate_id := ID_EMPTY

	for rows.Next() {
		tree = SkillsTree{}
		fmt.Println("rows: ", rows)

		err = rows.Scan(
			&cv.Id, &cv.Gpa, &cv.Yoe,
			&cv.JobRole.Id, &cv.JobRole.Name, &cv.Graduate.Id, &cv.Graduate.Username,
			&tree.JobSkill.Id, &tree.JobSkill.Name, &tree.Id,
		)

		if err != nil {
			log.Println(err.Error(), " ---> rows.Scan() error ---> ", sqlStmt)
			continue
		}

		cv.GraduateId = cv.Graduate.Id
		cv.JobRoleId = cv.JobRole.Id
		tree.Graduate = cv.Graduate
		tree.GraduateId = tree.Graduate.Id
		tree.JobSkillId = tree.JobSkill.Id

		cv.Tree = append(cv.Tree, tree)
		// fmt.Println("select rows for each cv : ", cv)

		if graduate_id != cv.GraduateId && graduate_id != ID_EMPTY {
			cvs = append(cvs, prevCv)
			cv.Tree = append([]SkillsTree{}, tree)

			graduate_id = cv.GraduateId
			fmt.Println("graduate_id changed !!!")
		}

		prevCv = cv
		graduate_id = cv.GraduateId
	}

	cvs = append(cvs, cv)

	err = rows.Err()
	if err != nil {
		log.Println(err.Error(), " ---> ", sqlStmt)

		return nil, err
	}

	return cvs, nil
}

func sendGmailNotification() (err error) {

	return err
}
