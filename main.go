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

type User struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

func main() {
	app := fiber.New()

	setupRoute(app)

	port := ":2200"
	app.Listen(port)
}

var secret_key string = "hello"

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
	api.Post("/login", func(c *fiber.Ctx) error {
		user := User{}

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

		return nil
	})
}

func jwtMiddlewareProtect(c *fiber.Ctx) error {
	token_string := extractTokenFromAuthHeader(c)
	fmt.Printf("Token String = %v \n", token_string)

	claims := jwt.MapClaims{}
	token, err := jwt.ParseWithClaims(token_string, claims, func(token *jwt.Token) (interface{}, error) {
		return []byte(secret_key), nil
	})

	if err != nil || !token.Valid {
		return err
	}

	fmt.Println("Claims = ")
	fmt.Println(claims)

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
