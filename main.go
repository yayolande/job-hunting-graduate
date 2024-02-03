package main

import (
	"database/sql"
	"fmt"
	"log"
	"net/smtp"
	"regexp"
	"strconv"
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/golang-jwt/jwt/v5"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/joho/godotenv"
	_ "github.com/mattn/go-sqlite3"
)

type UserCredential struct {
	Username string `json:"username,omitempty"`
	Password string `json:"password,omitempty"`
	Email    string `json:"email,omitempty"`
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

func (u User) IsMandatoryFieldFilled() bool {
	if len(u.Username) <= 0 || len(u.Password) <= 0 || len(u.Email) <= 0 {
		fmt.Println("Empty field Detected for user credential")
		fmt.Println(u)
		return false
	}

	emailRegex := `^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`
	match, err := regexp.MatchString(emailRegex, u.Email)
	if err != nil {
		fmt.Println("Error: ", err.Error())
		return false
	}

	return match
}

// Job properties inspired by : https://www.indeed.com/viewjob?jk=5d43c4aa2edf6f41&tk=1hh1n8q22jkuc800&from=serp&vjs=3
type Job struct {
	Id           int        `json:"id"`
	Title        string     `json:"title"`
	Yoe          float64    `json:"yoe"`
	RoleId       int        `json:"role_id"`
	Role         JobRole    `json:"role" gorm:"foreignKey:RoleId"`
	Tree         []JobSkill `json:"tree" gorm:"many2many:job_skills_tree"`
	IsRecruiting bool       `json:"is_recruiting" gorm:"default:true"`
	// Careful, this field must remain private (non-exported), otherwise it will break GORM functionalities. On the other and, this field must be in the same package as the db operation it is related with
	// Status         bool     `json:"status"`
	// Description    string   `json:"description"`
	// Skills         []string `json:"skills"` // Skills & Year of experience (optional)
	// Salary         []int    `json:"salary"` // Min & Max
	// Company        string   `json:"company"`
	// City           string   `json:"city"`
}

func (j Job) isValid() error {
	var err error = nil

	if j.Title == "" {
		err = fmt.Errorf("Job title and role are mandatory when creating new Job")
		return fiber.ErrBadGateway
	}

	if j.Yoe <= 0 || j.RoleId <= 0 {
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

	gormDB.Where("id = ? AND is_recruiting = true", j.JobId).Find(&jobs)
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
	Id         int        `json:"id"`
	Gpa        float64    `json:"gpa"`
	Yoe        float64    `json:"yoe"`
	GraduateId int        `json:"graduate_id"`
	JobRoleId  int        `json:"job_role_id"`
	Graduate   User       `json:"user" gorm:"foreignKey:GraduateId"`
	JobRole    JobRole    `json:"job_role" gorm:"foreignKey:JobRoleId"`
	Tree       []JobSkill `json:"tree" gorm:"many2many:graduate_skills_tree"`
}

type SkillsTree struct {
	Id         int `json:"id"`
	JobSkillId int `json:"job_skill_id"`
	CVId       int `json:"cv_id"`
}

func main() {
	var envApp map[string]string
	envApp, err := godotenv.Read("./.env")
	env = envApp

	if err != nil {
		log.Println("Error loading the .env file. ", err.Error())
		log.Println("As a consequence, no email notification will be sent to new registered users")
	}

	fmt.Println(envApp)

	// 1 -- Database Definition
	// os.Remove("./jobs.gormDb")

	gormDb, err := gorm.Open(sqlite.Open("./jobs.db"), &gorm.Config{})
	if err != nil {
		log.Fatal("Upcoming error")
		panic("failed to connect to the database")
	}

	printError := func(err error) {
		if err != nil {
			fmt.Println(err.Error())
		}
	}

	gormDB = gormDb
	// gormDB.Migrator().DropTable(&Job{})
	err = gormDb.AutoMigrate(&Job{})
	printError(err)
	err = gormDb.AutoMigrate(&JobRole{})
	printError(err)
	err = gormDb.AutoMigrate(&JobSkill{})
	printError(err)
	err = gormDb.AutoMigrate(&User{})
	printError(err)
	err = gormDb.AutoMigrate(&JobApplication{})
	printError(err)
	err = gormDb.AutoMigrate(&Friendship{})
	printError(err)
	err = gormDb.AutoMigrate(&Message{})
	printError(err)
	err = gormDb.AutoMigrate(&CurriculumVitae{})
	printError(err)

	db, err := sql.Open("sqlite3", "./jobs.db")
	if err != nil {
		fmt.Println("Unable to open Database. Error : ", err.Error())
		return
	}

	DB = db

	// 2 -- Launching the server
	app := fiber.New()
	setupRoute(app)

	port := ":2200"
	app.Listen(port)
}

var (
	secret_key string = "hello"
	gormDB     *gorm.DB
	DB         *sql.DB
	env        map[string]string
)

func setupRoute(app *fiber.App) {
	app.Use(func(c *fiber.Ctx) error {
		fmt.Println("Hello From CORS policy manager handler !")

		method := c.Route().Method
		fmt.Println("Method: ", method)
		c.Set("Access-Control-Allow-Origin", "*")
		c.Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		c.Set("Access-Control-Allow-Credentials", "true")
		c.Set("Access-Control-Allow-Headers", "Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization, accept, origin, Cache-Control, X-Requested-With")

		if method == "OPTIONS" {
			fmt.Println("OPTIONS is OK")
			return c.SendStatus(fiber.StatusNoContent)
		}

		return c.Next()
	})

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

		fmt.Println(user)

		if !user.IsMandatoryFieldFilled() {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"message": "User credential haven't been filled properly",
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

		err := sendGmailNotification(user.Email, user.Username, user.Password)
		if err != nil {
			fmt.Println("Failed to send mail ? ---> ", err.Error())
			// return c.SendStatus(fiber.StatusInternalServerError)
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

		gormDB.Model(&Job{}).
			Where("is_recruiting = true").
			Preload("Tree").
			Preload("Role").
			Find(&availableJobs)

		/*
			for _, el := range availableJobs {
				el.tree = []JobSkill{{Name: "oreimo"}}
			}
		*/

		fmt.Println(availableJobs)

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

	api.Get("/jobs/filtered/:my_id<int>?", func(c *fiber.Ctx) error {
		var passport UserPassport = getUserPassportFromMiddlewareContext(c)
		var param string = c.Params("my_id")

		user_id, err := strconv.Atoi(param)
		if err != nil {
			user_id = passport.Id
		}

		cv := CurriculumVitae{}
		err = gormDB.
			Preload("Graduate").
			Preload("JobRole").
			Preload("Tree").
			Where("graduate_id = ?", user_id).
			First(&cv).Error

		if err != nil {
			fmt.Println("Error while loading user CV: ", err.Error())
			return c.Status(fiber.StatusOK).JSON(fiber.Map{
				"message": err.Error(),
			})
		}

		jobs := []Job{}
		err = gormDB.
			Where("is_recruiting = true").
			Preload("Role").
			Preload("Tree").
			Find(&jobs).Error

		if err != nil {
			fmt.Println("Error while laod Job from DB: ", err.Error())
			return c.Status(fiber.StatusOK).JSON(fiber.Map{
				"message": err.Error(),
			})
		}

		filteredJobs := filterJobsByElligibility(cv, jobs)

		return c.Status(fiber.StatusOK).JSON(fiber.Map{
			"jobs": filteredJobs,
		})
	})

	api.Post("/jobs/skills", func(c *fiber.Ctx) error {
		type JobSkillTree struct {
			Job_id   int `json:"job_id"`
			Skill_id int `json:"job_skill_id"`
		}
		skillTree := JobSkillTree{}

		if err := c.BodyParser(&skillTree); err != nil {
			fmt.Println(err.Error())
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"message": err.Error(),
			})
		}

		err := saveJobSkillsToDB(DB, skillTree.Job_id, skillTree.Skill_id)
		if err != nil {
			fmt.Println(err.Error())
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"message": err.Error(),
			})
		}

		return c.Status(fiber.StatusOK).JSON(fiber.Map{
			"tree": skillTree,
		})
	})

	api.Post("/jobs/close/", func(c *fiber.Ctx) error {
		job := Job{}

		if err := c.BodyParser(&job); err != nil {
			fmt.Println("data parsing error: ", err.Error())

			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"message": err.Error(),
			})
		}

		jobSelector := Job{Id: job.Id}
		err := gormDB.
			Model(&jobSelector).
			Update("is_recruiting", job.IsRecruiting).
			Error
		if err != nil {
			fmt.Println("DB error: ", err.Error())
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"message": err.Error(),
			})
		}

		return c.Status(fiber.StatusOK).JSON(fiber.Map{
			"job_update": job,
		})
	})

	api.Get("/jobs/hidden", func(c *fiber.Ctx) error {
		hiddenJobs := []Job{}

		err := gormDB.
			Where("is_recruiting = false").
			Find(&hiddenJobs).Error
		if err != nil {
			fmt.Println("DB error: ", err.Error())
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"message": err.Error(),
			})
		}

		return c.Status(fiber.StatusOK).JSON(fiber.Map{
			"jobs": hiddenJobs,
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

	api.Get("/user/graduate/filtered/:my_id<int>?", func(c *fiber.Ctx) error {
		passport := getUserPassportFromMiddlewareContext(c)
		param := c.Params("my_id")
		user_id, err := strconv.Atoi(param)

		if err != nil {
			user_id = passport.Id
		}

		cv := CurriculumVitae{}
		err = gormDB.
			Preload("Graduate").
			Preload("JobRole").
			Preload("Tree").
			Where("graduate_id = ?", user_id).
			First(&cv).Error

		if err != nil {
			fmt.Println("CV Db fetch error: ", err.Error())
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"message": err.Error(),
			})
		}

		graduatesCvs := []CurriculumVitae{}
		err = gormDB.
			Preload("Graduate").
			Preload("JobRole").
			Preload("Tree").
			Where("graduate_id <> ?", user_id).
			Find(&graduatesCvs).Error

		if err != nil {
			fmt.Println("CVS Db fetch error: ", err.Error())
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"message": err.Error(),
			})
		}

		filteredCvs := filterGraduatesByCvToFindPotentialFriends(cv, graduatesCvs)

		return c.Status(fiber.StatusOK).JSON(fiber.Map{
			"cvs": filteredCvs,
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

	// TODO: Enforce the id parameter to be <int> (":my_id<int>", "friend_id<int>")
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
		cvs := []CurriculumVitae{}

		err := gormDB.Model(&CurriculumVitae{}).
			Preload("Graduate").
			Preload("JobRole").
			Preload("Tree").
			Find(&cvs).Error
		// cvs, err := getAllGraduateCurriculumViateFromDB(DB)

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

	api.Get("/cv/:my_id<int>", func(c *fiber.Ctx) error {
		cvId := c.Params("my_id")

		cv := CurriculumVitae{}

		err := gormDB.Where("id = ?", cvId).
			Preload("Graduate").
			Preload("JobRole").
			Preload("Tree").
			First(&cv).Error

		if err != nil {
			fmt.Println("DB error: ", err.Error())
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"message": err.Error()})
		}

		return c.Status(fiber.StatusOK).JSON(fiber.Map{
			"cv": cv,
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

		err := gormDB.Create(&cv).Error
		// err := saveCurriculumVitae(DB, cv)
		if err != nil {
			message := "Unable to save the data to database"
			log.Println(message, " ---> ", err.Error())
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"message": message,
			})
		}

		return c.Status(fiber.StatusOK).JSON(fiber.Map{
			"cv": cv,
		})
	})

	/*
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
	*/

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
		skills := []JobSkill{}

		err := gormDB.Find(&skills).Error

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

		err := gormDB.Create(&skill).Error
		// err := saveSkillsToDB(DB, skill)
		if err != nil {
			return c.SendStatus(fiber.StatusInternalServerError)
		}

		return c.Status(fiber.StatusOK).JSON(fiber.Map{
			"skill": skill,
		})
	})

	api.Get("/job_roles", func(c *fiber.Ctx) error {
		roles := []JobRole{}

		err := gormDB.Find(&roles).Error
		// roles, err := getJobRoleFromDB(DB)

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

		err := gormDB.Create(&role).Error
		// err := saveJobRoleToDB(DB, role)
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

func sendGmailNotification(emailReceiver string, username string, userpass string) (err error) {
	password := env["GMAIL_PASSWORD"]
	sender := env["GMAIL_ACCOUNT"]
	// password := env["YAHOO_PASSWORD"]
	// sender := env["YAHOO_ACCOUNT"]
	receiver := []string{emailReceiver}

	subject := "Registration to Job Platform for Graduate Complete"
	body := "We are happy to count you in ! This Platform is a thriving community." +
		" for quickstarting your carreer" + "\r\n Here are your credentials:\r\n"

	message := []byte(
		"To: " + receiver[0] +
			"\r\nSubject: " + subject +
			"\r\n\r\n" + body +
			"\r\n" + "Username: " + username +
			"\r\n" + "Password: " + userpass,
	)

	host := "smtp.gmail.com"
	port := "587"
	// host := "smtp.mail.yahoo.com"
	// port := "465"
	smtpServer := host + ":" + port

	auth := smtp.PlainAuth("", sender, password, host)
	err = smtp.SendMail(smtpServer, auth, sender, receiver, message)

	if err != nil {
		log.Println("Failed to send the email notification. Error : ", err.Error())
	}

	return err
}

// ================================================================================================
// ================================================================================================
// ==================== Manual Interaction with the DB using standard lib =========================
// ================================================================================================
// ================================================================================================

func saveJobSkillsToDB(db *sql.DB, job_id int, skill_id int) (err error) {
	var sqlStmt string = `
  INSERT INTO job_skills_tree (job_id, job_skill_id)
  VALUES (?, ?);
  `

	_, err = db.Exec(sqlStmt, job_id, skill_id)
	if err != nil {
		log.Println(err.Error(), " ---> ", sqlStmt)
	}

	return err
}

func saveSkillsTreeToDB(db *sql.DB, tree SkillsTree) (err error) {
	var sqlStmt string = `
    INSERT INTO graduate_skills_tree (curriculum_vitae_id, job_skill_id)
    VALUES (?, ?);
  `
	_, err = db.Exec(sqlStmt, tree.CVId, tree.JobSkillId)
	if err != nil {
		log.Println(err.Error(), " ---> ", sqlStmt)
	}

	return err
}

func filterJobsByElligibility(userCv CurriculumVitae, availableJobs []Job) []Job {
	filteredJobs := []Job{}
	points := 0.0

	for _, job := range availableJobs {
		if userCv.Gpa > 2.5 {
			points = (userCv.Gpa - 2.5) * 10
		}

		if userCv.JobRoleId == job.RoleId {
			points += 10
			points += userCv.Yoe * 5
		}

		for _, jobSkill := range job.Tree {
			for _, cvSkill := range userCv.Tree {
				if jobSkill.Id == cvSkill.Id {
					points += 3
					fmt.Println("Gained points for jobskill: ", jobSkill)
				}
			}

		}

		if points >= 15.0 {
			filteredJobs = append(filteredJobs, job)
		}
		fmt.Println("filber job ? Job = ", job, " ---> points = ", points)
	}

	return filteredJobs
}

func filterGraduatesByCvToFindPotentialFriends(userCv CurriculumVitae, graduatesCvs []CurriculumVitae) []CurriculumVitae {
	filteredCvs := []CurriculumVitae{}
	points := 0.0

	for _, cv := range graduatesCvs {
		if cv.Gpa > 2.5 {
			points = (cv.Gpa - 2.5) * 10
		}

		if cv.JobRoleId == userCv.JobRoleId {
			points += 10
			points += cv.Yoe * 5
		}

		for _, jobSkill := range cv.Tree {
			for _, cvSkill := range userCv.Tree {
				if jobSkill.Id == cvSkill.Id {
					points += 3
					fmt.Println("Gained points for jobskill: ", jobSkill)
				}
			}

		}

		if points >= 10.0 {
			filteredCvs = append(filteredCvs, cv)
		}
		fmt.Println("filber cv ? cv = ", cv, " ---> points = ", points)
	}

	return filteredCvs
}

/*

func databaseExec(db *sql.DB, sqlStmt string) error {
	_, err := db.Exec(sqlStmt)
	if err != nil {
		log.Print("[DB error] ", err.Error(), " \n --> For SQL query : ", sqlStmt)
		return err
	}

	return nil
}

func saveSkillsToDB(db *sql.DB, skill JobSkill) (err error) {
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
		// fmt.Println("select rows for each cv : ", cv)

		if graduate_id != cv.GraduateId && graduate_id != ID_EMPTY {
			cvs = append(cvs, prevCv)
			cv.Tree = append([]SkillsTree{}, tree)

			graduate_id = cv.GraduateId
			// fmt.Println("graduate_id changed !!!")
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

	fmt.Println("cvs : ", cvs)
	return cvs, nil
}

*/
