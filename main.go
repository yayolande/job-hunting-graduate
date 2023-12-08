package main

import (
	"fmt"
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/golang-jwt/jwt/v5"
)

type inputJson struct {
	Name string `json:"name"`
	City string `json:"city"`
}

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

func main() {
	app := fiber.New()

	setupRoute(app)

	port := ":2200"
	app.Listen(port)
}

var secret_key string = "hello"
var users = []User{
	{
		Credential: UserCredential{"admin", "admin"},
		Passport:   UserPassport{1, true, false, false},
	},
	{
		Credential: UserCredential{"steveen", "password"},
		Passport:   UserPassport{2, false, true, false},
	},
	{
		Credential: UserCredential{"graduate", "graduate"},
		Passport:   UserPassport{3, false, true, false},
	},
	{
		Credential: UserCredential{"Mojo Corp.", "mojo"},
		Passport:   UserPassport{4, false, false, true},
	},
	{
		Credential: UserCredential{"employer", "employer"},
		Passport:   UserPassport{5, false, false, true},
	},
}

var _jobs = []string{"JOb 1", "Job 2", "Job 3", "Job 4", "Job 5"}
var jobs = []Job{
	{0, "Need of Software Engineer", "Software Developer", 2},
	{1, "Hospital need Doctor", "Head Doctor", 5},
	{1, "Beer Company need finance officer", "Finance Director", 10},
}

func setupRoute(app *fiber.App) {
	app.Get("/", func(c *fiber.Ctx) error {
		// text := "Hello World !"
		input := inputJson{}
		notOkay := c.BodyParser(&input)
		if notOkay != nil {
			return c.Status(fiber.StatusBadRequest).Send([]byte(notOkay.Error()))
		}

		// return c.Status(fiber.StatusOK).SendString(text)
		return c.Status(fiber.StatusOK).JSON(fiber.Map{
			"name": input.Name + " " + input.City,
			"city": input.City,
		})
	})

	api := app.Group("/api/v1")

	// Note: JWT Token should only contains data necessary for authorization
	// on the server. The remaining data can be passed normally

	// TODO: Registration API path

	api.Post("/login", func(c *fiber.Ctx) error {
		user := UserCredential{}

		if err := c.BodyParser(&user); err != nil {
			c.Status(fiber.StatusBadGateway).JSON(fiber.Map{
				"message": err.Error(),
			})
		}

		// Check User in DB
		userPassport, err := getUserPassportFromCredential(user)

		if err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"message": err.Error(),
			})
		}

		// If found, send token back to client
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

	api.Post("/test/login", func(c *fiber.Ctx) error {
		user := UserCredential{}

		if err := c.BodyParser(&user); err != nil {
			c.Status(fiber.StatusBadGateway).Send([]byte(err.Error()))
		}

		// Check User in DB

		// If found, send token back to client
		claims := jwt.MapClaims{
			"username": user.Username,
			"password": user.Password,
			// "exp":      time.Now().Add(time.Minute * 5).Unix(),
		}

		key := []byte(secret_key)
		token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
		token_string, _ := token.SignedString(key)

		return c.Status(fiber.StatusOK).JSON(fiber.Map{
			"token": token_string,
			"users": users,
		})
	})

	api.Get("/test/static_token_extraction", func(c *fiber.Ctx) error {
		token_string := "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJleHAiOjE3MDE2MDg2MjQsInBhc3N3b3JkIjoiZWxpb3QiLCJ1c2VybmFtZSI6InN0ZXZlZW5zb24ifQ.mMvXGCSlrhEoidprn8GxW5DpJWWtq_DXD9i4uLPUR4U"

		type Token_Info struct {
			Username string `json:"username"`
			Password string `json:"password"`
			jwt.RegisteredClaims
		}

		_claims := Token_Info{}
		token, err := jwt.ParseWithClaims(token_string, &_claims, func(token *jwt.Token) (interface{}, error) {
			return []byte(secret_key), nil
		})

		if err != nil || !token.Valid {
			fmt.Println(err)
			return c.Status(fiber.StatusBadRequest).JSON(err.Error())
		}

		return c.Status(fiber.StatusOK).JSON(fiber.Map{
			"_token": _claims,
		})

	})

	api.Get("/test/middlewareProtect", jwtMiddlewareProtect, func(c *fiber.Ctx) error {
		fmt.Print("Inside Middleware Protect TEst")

		return c.Status(fiber.StatusOK).JSON(fiber.Map{
			"users": c.Locals("users"),
		})
	})

	api.Use(jwtMiddlewareProtect)

	api.Get("/jobs", graduateOnlyMiddleware, func(c *fiber.Ctx) error {
		// Return all list of available job, if an user is a graduate
		// In case of not being a graduate, return an error
		return c.Status(fiber.StatusOK).JSON(fiber.Map{
			"jobs": jobs,
		})
		// return nil
	})
}

func getUserPassportFromCredential(client UserCredential) (*UserPassport, error) {
	for _, user := range users {
		credential := user.Credential

		if credential.Username == client.Username && credential.Password == client.Password {
			// TODO: Need better implementation
			return &user.Passport, nil // This one is not good, what if later I modify user, it will also affect the database
		}
	}

	var err error = fmt.Errorf("User not found")

	return nil, err
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
	c.Locals("users", users) // Only here for testing in "/jobs" path, not necessary
	c.Locals("user", users[1])
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
