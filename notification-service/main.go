package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"os"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"

	pb "github.com/technonext/todo-app/proto/proto"
)

type server struct {
	pb.UnimplementedNotificationServiceServer
	collection *mongo.Collection
}

type Notification struct {
	ID        primitive.ObjectID `bson:"_id,omitempty"`
	UserID    string             `bson:"user_id"`
	Message   string             `bson:"message"`
	Read      bool               `bson:"read"`
	CreatedAt string             `bson:"created_at"`
}

func (s *server) SendNotification(ctx context.Context, req *pb.NotificationRequest) (*pb.NotificationResponse, error) {
	now := time.Now().Format(time.RFC3339)
	notification := Notification{
		UserID:    req.UserId,
		Message:   req.Message,
		Read:      false,
		CreatedAt: now,
	}

	result, err := s.collection.InsertOne(ctx, notification)
	if err != nil {
		log.Printf("Failed to create notification: %v", err)
		return nil, err
	}

	oid, ok := result.InsertedID.(primitive.ObjectID)
	if !ok {
		log.Printf("Failed to convert ObjectID")
		return nil, err
	}

	return &pb.NotificationResponse{
		Notification: &pb.Notification{
			Id:        oid.Hex(),
			UserId:    notification.UserID,
			Message:   notification.Message,
			Read:      notification.Read,
			CreatedAt: notification.CreatedAt,
		},
	}, nil
}

func (s *server) GetNotifications(ctx context.Context, req *pb.GetNotificationsRequest) (*pb.GetNotificationsResponse, error) {
	filter := bson.M{"user_id": req.UserId}
	if req.UnreadOnly {
		filter["read"] = false
	}

	findOptions := options.Find()
	findOptions.SetLimit(int64(req.Limit))
	findOptions.SetSkip(int64(req.Page * req.Limit))
	findOptions.SetSort(bson.D{{"created_at", -1}})

	cursor, err := s.collection.Find(ctx, filter, findOptions)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var notifications []*pb.Notification
	for cursor.Next(ctx) {
		var notification Notification
		if err := cursor.Decode(&notification); err != nil {
			return nil, err
		}
		notifications = append(notifications, &pb.Notification{
			Id:        notification.ID.Hex(),
			UserId:    notification.UserID,
			Message:   notification.Message,
			Read:      notification.Read,
			CreatedAt: notification.CreatedAt,
		})
	}

	if err := cursor.Err(); err != nil {
		return nil, err
	}

	count, err := s.collection.CountDocuments(ctx, filter)
	if err != nil {
		return nil, err
	}

	return &pb.GetNotificationsResponse{
		Notifications: notifications,
		Total:         int32(count),
	}, nil
}

func main() {
	// Read the environment variables
	mongoUser := os.Getenv("MONGO_USERNAME")
	mongoPass := os.Getenv("MONGO_PASSWORD")
	mongoHost := os.Getenv("MONGO_HOST")
	if mongoUser == "" || mongoPass == "" || mongoHost == "" {
		log.Fatal("Error: MONGO_USERNAME, MONGO_PASSWORD, and MONGO_HOST must be set")
	}
	// Build the connection string
	mongoURI := fmt.Sprintf("mongodb://%s:%s@%s/todo_app?authSource=admin", mongoUser, mongoPass, mongoHost)

	log.Printf("Connecting to MongoDB at %s...", mongoHost)
	// Connect to MongoDB
	client, err := mongo.Connect(context.Background(), options.Client().ApplyURI(mongoURI))
	if err != nil {
		log.Fatalf("Failed to connect to MongoDB: %v", err)
	}
	defer client.Disconnect(context.Background())

	// Check the connection
	err = client.Ping(context.Background(), nil)
	if err != nil {
		log.Fatalf("Failed to ping MongoDB: %v", err)
	}

	collection := client.Database("todo_app").Collection("notifications")

	// Get port from environment variable
	port := os.Getenv("PORT")
	if port == "" {
		port = "50053"
	}

	lis, err := net.Listen("tcp", ":"+port)
	if err != nil {
		log.Fatalf("Failed to listen: %v", err)
	}

	s := grpc.NewServer()
	pb.RegisterNotificationServiceServer(s, &server{collection: collection})
	reflection.Register(s)

	log.Printf("Notification service listening on port %s", port)
	if err := s.Serve(lis); err != nil {
		log.Fatalf("Failed to serve: %v", err)
	}
}
