package migrations

import (
	"context"

	"github.com/sirupsen/logrus"
	migrate "github.com/xakep666/mongo-migrate"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
)

var migration29 = migrate.Migration{
	Version:     29,
	Description: "add last_login field to collection users",
	Up: func(db *mongo.Database) error {
		logrus.Info("Applying migration 29 - Up")
		if _, err := db.Collection("users").UpdateMany(context.TODO(), bson.M{}, bson.M{"$set": bson.M{"last_login": nil}}); err != nil {
			return err
		}

		return nil
	},
	Down: func(db *mongo.Database) error {
		logrus.Info("Applying migration 29 - Down")

		return nil
	},
}
