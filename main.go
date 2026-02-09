package main

import (
	"context"
	"encoding/base64"
	"log"
	"os"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/joho/godotenv"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

var (
	col *mongo.Collection

	pixelBytes, _ = base64.StdEncoding.DecodeString(
		"R0lGODlhAQABAPAAAP///wAAACH5BAAAAAAALAAAAAABAAEAAAICRAEAOw==",
	)
)

func main() {
	_ = godotenv.Load()

	mongoURI := os.Getenv("MONGO_URI")
	dbName := os.Getenv("MONGO_DB")
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	if mongoURI == "" || dbName == "" {
		log.Fatal("MONGO_URI or MONGO_DB missing")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	client, err := mongo.Connect(ctx, options.Client().ApplyURI(mongoURI))
	if err != nil {
		log.Fatal(err)
	}

	col = client.Database(dbName).Collection("email_tracking")

	app := fiber.New()

	app.Post("/emails", createEmail)
	app.Get("/pixel", trackOpen)

	log.Fatal(app.Listen(":" + port))
}

// POST /emails
func createEmail(c *fiber.Ctx) error {
	var payload map[string]interface{}

	if err := c.BodyParser(&payload); err != nil {
		return c.SendStatus(fiber.StatusBadRequest)
	}

	emailID, ok := payload["email_id"].(string)
	if !ok || emailID == "" {
		return c.SendStatus(fiber.StatusBadRequest)
	}

	now := time.Now()

	update := bson.M{
		"$setOnInsert": bson.M{
			"email_id":   emailID,
			"sent_at":    now,
			"opened":     false,
			"open_count": 0,
		},
		"$set": payload, // store full payload exactly as sent
	}

	_, err := col.UpdateOne(
		c.Context(),
		bson.M{"email_id": emailID},
		update,
		options.Update().SetUpsert(true),
	)

	if err != nil {
		return c.SendStatus(500)
	}

	return c.SendStatus(fiber.StatusCreated)
}

// GET /pixel?id=<email_id>
func trackOpen(c *fiber.Ctx) error {
	emailID := c.Query("id")
	if emailID == "" {
		return c.SendStatus(fiber.StatusBadRequest)
	}

	now := time.Now()

	_, _ = col.UpdateOne(
		c.Context(),
		bson.M{"email_id": emailID},
		bson.M{
			"$set": bson.M{
				"opened":   true,
				"last_ip":  c.IP(),
				"last_ua":  c.Get("User-Agent"),
				"last_hit": now,
			},
			"$inc": bson.M{
				"open_count": 1,
			},
		},
	)

	c.Set("Content-Type", "image/gif")
	c.Set("Cache-Control", "no-store, no-cache, must-revalidate, max-age=0")
	return c.Send(pixelBytes)
}
