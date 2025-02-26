package main

import (
	"bytes"
	"context"
	"fmt"
	"image/color"
	"log"
	"math/rand"
	"net/http"
	"time"

	"github.com/fogleman/gg"
	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
)

// Redis context
var ctx = context.Background()
var redisClient *redis.Client

func main() {
	// Initialize Redis
	redisClient = redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
	})
	if err := redisClient.Ping(ctx).Err(); err != nil {
		log.Fatal("Failed to connect to Redis:", err)
	}

	fmt.Println("Connected to Redis successfully")

	// Setup Gin router
	r := gin.Default()

	r.GET("/captcha", generateCaptcha)
	r.POST("/captcha/verify", verifyCaptcha)

	// Start server
	r.Run(":8080")
}

// API handler to generate CAPTCHA
func generateCaptcha(c *gin.Context) {
	// Generate a random CAPTCHA text
	captchaText := generateRandomString(5)
	captchaID := fmt.Sprintf("captcha_%d", time.Now().UnixNano())

	// Store the correct solution in Redis
	redisClient.Set(ctx, captchaID, captchaText, 5*time.Minute)

	// Generate CAPTCHA image
	imageData, err := generateCaptchaImg(captchaText)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate CAPTCHA"})
		return
	}

	// Return CAPTCHA ID and image
	c.Header("X-Captcha-ID", captchaID)
	fmt.Println("X-Captcha-ID", captchaID)
	c.Data(http.StatusOK, "image/png", imageData)
}

// Generate a random string for CAPTCHA
func generateRandomString(length int) string {
	letters := "ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	result := make([]byte, length)
	for i := range result {
		result[i] = letters[rand.Intn(len(letters))]
	}
	return string(result)
}

// Generate a CAPTCHA image
func generateCaptchaImg(text string) ([]byte, error) {
	const width, height = 200, 80
	dc := gg.NewContext(width, height)

	// Set background color
	dc.SetColor(color.White)
	dc.Clear()

	// Set text color
	dc.SetRGB(0, 0, 0)
	dc.LoadFontFace("Chalkduster.ttf", 36)
	dc.DrawStringAnchored(text, width/2, height/2, 0.5, 0.5)

	// Save image to buffer
	var buf bytes.Buffer
	err := dc.EncodePNG(&buf)
	if err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// API handler to verify CAPTCHA
func verifyCaptcha(c *gin.Context) {
	var request struct {
		CaptchaID  string `json:"captcha_id"`
		UserAnswer string `json:"user_answer"`
	}

	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
		return
	}

	// Get the stored solution from Redis
	storedCaptcha, err := redisClient.Get(ctx, request.CaptchaID).Result()
	if err != nil || storedCaptcha == "" {
		c.JSON(http.StatusForbidden, gin.H{"error": "CAPTCHA expired or invalid"})
		return
	}

	// Check if the answer matches
	if storedCaptcha != request.UserAnswer {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Incorrect CAPTCHA"})
		return
	}

	// Success!
	c.JSON(http.StatusOK, gin.H{"message": "CAPTCHA verified"})
}
