package main

import (
	"crypto/rand"
	"fmt"
	"log"
	"net/http"
	"net/smtp"
	"os"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/joho/godotenv"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

type Database struct {
	Host     string
	User     string
	Password string
	DbName   string
	Port     int
}

func connectDB() *gorm.DB {

	err := godotenv.Load()
	if err != nil {
		log.Fatal("Error loading .env file")
	}

	infor := Database{
		Host:     os.Getenv("DB_HOST"),
		User:     os.Getenv("DB_USER"),
		Password: os.Getenv("DB_PASSWORD"),
		DbName:   os.Getenv("DB_NAME"),
		Port:     5432,
	}

	dsn := fmt.Sprintf("host=%s user=%s password=%s dbname=%s port=%d sslmode=disable TimeZone=Asia/Shanghai",
		infor.Host, infor.User, infor.Password, infor.DbName, infor.Port)

	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		log.Fatal("Failed to connect to the database:", err)
	}
	fmt.Println("Connected to DB successfully!")
	return db
}

type OTPCode struct {
	ID        int       `gorm:"primaryKey"`
	OTPCode   string    `gorm:"column:otp_code"`
	CreatedAt time.Time `gorm:"column:created_at"`
	ExpiresAt time.Time `gorm:"column:expires_at"`
}

type OtpCodes struct {
	OTPCode   string    `gorm:"column:otp_code"`
	CreatedAt time.Time `gorm:"column:created_at"`
	ExpiresAt time.Time `gorm:"column:expires_at"`
}

type Response struct {
	StatusCode int         `json:"statusCode"`
	Message    string      `json:"message"`
	Data       interface{} `json:"data"`
}

const otpChars = "1234567890"

func GenerateOTP(length int) (string, error) {
	buffer := make([]byte, length)
	_, err := rand.Read(buffer)
	if err != nil {
		return "", err
	}
	otpCharsLength := len(otpChars)
	for i := 0; i < length; i++ {
		buffer[i] = otpChars[int(buffer[i])%otpCharsLength]
	}

	return string(buffer), nil
}

func sendEmail(otp string) {

	from := os.Getenv("SMTP_EMAIL")
	pass := os.Getenv("SMTP_PASSWORD")
	to := os.Getenv("TO_EMAIL")

	msg := "From: " + from + "\n" +
		"To: " + to + "\n" +
		"Subject: OTP Verification\n\n" +
		"Your OTP is: " + otp + "\n" +
		"Please enter the OTP to unlock the system."

	err := smtp.SendMail("smtp.gmail.com:587",
		smtp.PlainAuth("", from, pass, "smtp.gmail.com"),
		from, []string{to}, []byte(msg))

	if err != nil {
		log.Printf("smtp error: %s", err)
		return
	}
	fmt.Println("OTP sent successfully")
}

func messagePubHandler(db *gorm.DB) mqtt.MessageHandler {
	return func(client mqtt.Client, msg mqtt.Message) {

		fmt.Printf("Received message on topic %s: %s\n", msg.Topic(), msg.Payload())

		otp, err := GenerateOTP(6)
		if err != nil {
			log.Println("Error generating OTP:", err)
			return
		}

		sendEmail(otp)

		current := time.Now()
		expiresAt := current.Add(time.Minute * 5)
		otp_codes := OtpCodes{
			OTPCode:   otp,
			CreatedAt: current,
			ExpiresAt: expiresAt,
		}

		result := db.Create(&otp_codes)
		if result.Error != nil {
			log.Printf("Failed to save OTP in database: %v", result.Error)
			return
		}
	}
}

type Request struct {
	OTPCode string `json:"otp_code"`
}

func VerifyOTP(c echo.Context, db *gorm.DB, mqttClient mqtt.Client) error {
	request := Request{}
	if err := c.Bind(&request); err != nil {
		return c.JSON(http.StatusBadRequest, Response{
			StatusCode: http.StatusBadRequest,
			Message:    "Invalid input",
		})
	}

	otpRecord := OTPCode{}

	if err := db.Where("otp_code = ?", request.OTPCode).First(&otpRecord).Error; err != nil {
		mqttClient.Publish("otp/verification", 0, false, "OTP verification failed: Invalid OTP")
		return c.JSON(http.StatusUnauthorized, Response{
			StatusCode: http.StatusUnauthorized,
			Message:    "Invalid OTP",
		})
	}
	

	if time.Now().After(otpRecord.ExpiresAt) {
		mqttClient.Publish("otp/verification", 0, false, "OTP verification failed: OTP expired")
		return c.JSON(http.StatusUnauthorized, Response{
			StatusCode: http.StatusUnauthorized,
			Message:    "OTP expired",
		})
	}
	mqttClient.Publish("otp/verification", 0, false, "OTP verified successfully")
	return c.JSON(http.StatusOK, Response{
		StatusCode: http.StatusOK,
		Message:    "OTP verified successfully",
	})
}

func connectHandler() mqtt.OnConnectHandler {
	return func(client mqtt.Client) {
		fmt.Println("Connected to MQTT broker")
	}
}

func connectLostHandler() mqtt.ConnectionLostHandler {
	return func(client mqtt.Client, err error) {
		fmt.Printf("Connection lost: %v\n", err)
	}
}

func publish(client mqtt.Client) {
	token := client.Publish("openStatus", 0, false, "hello")
	token.Wait()
}

func sub(client mqtt.Client) {
	topic := "receive-signal"
	token := client.Subscribe(topic, 1, nil)
	token.Wait()
	fmt.Printf("Subscribed to topic: %s \n", topic)
}

func main() {
	e := echo.New()
	e.Use(middleware.CORS())
	db := connectDB()

	err := godotenv.Load()
	if err != nil {
		log.Fatal("Error loading .env file")
	}

	broker := os.Getenv("MQTT_BROKER_URL")
	port := 8883
	opts := mqtt.NewClientOptions()
	opts.AddBroker(fmt.Sprintf("tls://%s:%d", broker, port))
	opts.SetClientID(os.Getenv("MQTT_CLIENT_ID"))
	opts.SetUsername(os.Getenv("MQTT_USERNAME"))
	opts.SetPassword(os.Getenv("MQTT_PASSWORD"))
	opts.SetDefaultPublishHandler(messagePubHandler(db))
	opts.OnConnect = connectHandler()
	opts.OnConnectionLost = connectLostHandler()
	client := mqtt.NewClient(opts)

	if token := client.Connect(); token.Wait() && token.Error() != nil {
		panic(token.Error())
	}

	sub(client)
	publish(client)

	e.GET("/", func(c echo.Context) error {
		var otps []OTPCode
		if err := db.Find(&otps).Error; err != nil {
			log.Println("Error fetching records:", err)
			return c.JSON(http.StatusInternalServerError, Response{
				StatusCode: http.StatusInternalServerError,
				Message:    "Error fetching OTP records",
			})
		}
		return c.JSON(http.StatusOK, Response{
			StatusCode: http.StatusOK,
			Message:    "Fetched OTP records successfully",
			Data:       otps,
		})
	})

	e.POST("/verify-otp", func(c echo.Context) error {
		return VerifyOTP(c, db, client)
	})

	e.Logger.Fatal(e.Start(":3000"))
}
