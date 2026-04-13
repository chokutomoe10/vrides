package repository

import (
	"context"
	"fmt"
	"vrides/services/trip-service/internal/domain"
	"vrides/shared/db"
	pbd "vrides/shared/proto/driver"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

type mongoRepository struct {
	db *mongo.Database
}

func NewMongoRepository(db *mongo.Database) *mongoRepository {
	return &mongoRepository{db: db}
}

func (mr *mongoRepository) CreateTrip(ctx context.Context, trip *domain.TripModel) (*domain.TripModel, error) {
	result, err := mr.db.Collection(db.TripsCollection).InsertOne(ctx, trip)
	if err != nil {
		return nil, err
	}

	trip.ID = result.InsertedID.(primitive.ObjectID)

	return trip, nil
}

func (mr *mongoRepository) SaveRideFare(ctx context.Context, f *domain.RideFareModel) error {
	result, err := mr.db.Collection(db.RideFaresCollection).InsertOne(ctx, f)
	if err != nil {
		return err
	}

	f.ID = result.InsertedID.(primitive.ObjectID)

	return nil
}

func (mr *mongoRepository) GetRideFareByID(ctx context.Context, id string) (*domain.RideFareModel, error) {
	_id, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return nil, err
	}

	result := mr.db.Collection(db.RideFaresCollection).FindOne(ctx, bson.M{"_id": _id})
	if result.Err() != nil {
		return nil, result.Err()
	}

	var rf domain.RideFareModel
	if err := result.Decode(&rf); err != nil {
		return nil, err
	}

	return &rf, nil
}

func (mr *mongoRepository) GetTripByID(ctx context.Context, id string) (*domain.TripModel, error) {
	_id, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return nil, err
	}

	result := mr.db.Collection(db.TripsCollection).FindOne(ctx, bson.M{"_id": _id})
	if result.Err() != nil {
		return nil, result.Err()
	}

	var tm domain.TripModel
	if err := result.Decode(&tm); err != nil {
		return nil, err
	}

	return &tm, nil
}

func (mr *mongoRepository) UpdateTrip(ctx context.Context, tripID string, status string, driver *pbd.Driver) error {
	_id, err := primitive.ObjectIDFromHex(tripID)
	if err != nil {
		return err
	}

	update := bson.M{"$set": bson.M{"status": status}}

	if driver != nil {
		update["$set"].(bson.M)["driver"] = driver
	}

	result, err := mr.db.Collection(db.TripsCollection).UpdateOne(ctx, bson.M{"_id": _id}, update)
	if err != nil {
		return err
	}

	if result.ModifiedCount == 0 {
		return fmt.Errorf("trip not found: %s", tripID)
	}

	return nil
}
