package main

import (
	// "database/sql"
	"fmt"
	"log"
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/golang-jwt/jwt/v5"

	// _ "github.com/mattn/go-sqlite3"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

type UserCredential struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type UserPassport struct {
	Id       int  `json:"id"`
	Admin    bool `json:"admin"`
	Graduate bool `json:"graduate"`
	Employer bool `json:"employer"`
}

type User struct {
	Credential UserCredential `json:"credential"`
	Passport   UserPassport   `json:"passport"`
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

// =============================================
// =============================================
//              Table Definition
// =============================================
// =============================================

type UserTable struct {
	// gorm.Model
	UserPassport
	UserCredential
}

type JobTable struct {
	// gorm.Model
	Job
}

// =============================================
// =============================================
//              End Table Definition
// =============================================
// =============================================

func main() {

	// os.Remove("./jobs.db")

	db, err := gorm.Open(sqlite.Open("./jobs.db"), &gorm.Config{})
	if err != nil {
		log.Fatal("Upcoming error")
		panic("failed to connect to the database")
	}

	DB = db
	db.AutoMigrate(&Job{})
	db.AutoMigrate(&UserTable{})
	db.AutoMigrate(&JobTable{})

	// job := JobTable{
	// 	Title:          "Software Engineer",
	// 	Role:           "Software Engineer",
	// 	YearExperience: 2,
	// 	// Id: 1,
	// }

	// _job := JobTable{
	// 	Job: Job{
	// 		Title:          "Software Engineer",
	// 		Role:           "Software Engineer",
	// 		YearExperience: 2,
	// 		// Id: 1,
	// 	},
	// }

	// db.Create(&_job)

	// _job = JobTable{
	// 	Job{
	// 		Title: "Chef Cook",
	// 	},
	// }

	// db.Create(&_job)

	// _job = JobTable{
	// 	Job: Job{
	// 		Role:  "Cleaner",
	// 		Title: "ABC Company need a cleaner",
	// 	},
	// }

	// db.Create(&_job)

	// db.Create(&job)

	// job = Job{
	// 	Title:          "Cheater",
	// 	Role:           "Cheater",
	// 	YearExperience: 5,
	// 	// Id: 1,
	// }
	// db.Create(&job)

	var j1 []Job
	db.Find(&j1)

	fmt.Println(j1)

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
		user := &UserTable{}

		if err := c.BodyParser(user); err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"message": err.Error(),
			})
		}

		// Check that the user doesn't already exist before creating it
		existingUser := &[]UserTable{}
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
		existingUsers := []UserTable{}
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
		availableJobs := []JobTable{}

		DB.Find(&availableJobs)

		return c.Status(fiber.StatusOK).JSON(fiber.Map{
			"jobs": availableJobs,
		})
	})
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
	// return c.JSON(fiber.Map{
	// 	"user_passport": c.Locals("user_passport"),
	// })
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
	// Every token only have the UserPassword, not all the User type
	token_string := extractTokenFromAuthHeader(c)
	fmt.Printf("Token String = %v \n", token_string)

	type CustomClaims struct {
		jwt.RegisteredClaims
		Passport UserPassport `json:"passport"`
	}

	// _claims := jwt.MapClaims{}
	// token, err = jwt.ParseWithClaims(token_string, _claims, func(token *jwt.Token) (interface{}, error) {
	// 	return []byte(secret_key), nil
	// })
	// fmt.Println("Claims = ")
	// fmt.Println(_claims)

	// claims := jwt.MapClaims{}
	claims := &CustomClaims{}
	token, err := jwt.ParseWithClaims(token_string, claims, func(token *jwt.Token) (interface{}, error) {
		return []byte(secret_key), nil
	})

	if err != nil || !token.Valid {
		return err
	}

	fmt.Println("Claims = ")
	fmt.Println(claims)

	// TODO: Use "UserPassport" instead of "User"
	// c.Locals("users", users) // Only here for testing in "/jobs" path, not necessary
	// c.Locals("user", users[1])
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
