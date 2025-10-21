package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"os"
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.mongodb.org/mongo-driver/v2/bson"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"

	pb "github.com/technonext/todo-app/proto/proto"
)

type server struct {
	pb.UnimplementedAnalyticsServiceServer
	collection     *mongo.Collection
	taskCollection *mongo.Collection
}

type Event struct {
	ID         primitive.ObjectID `bson:"_id,omitempty"`
	UserID     string             `bson:"user_id"`
	EventType  string             `bson:"event_type"`
	ResourceID string             `bson:"resource_id"`
	Metadata   string             `bson:"metadata"`
	CreatedAt  string             `bson:"created_at"`
}

func (s *server) TrackEvent(ctx context.Context, req *pb.TrackEventRequest) (*pb.TrackEventResponse, error) {
	now := time.Now().Format(time.RFC3339)
	event := Event{
		UserID:     req.UserId,
		EventType:  req.EventType,
		ResourceID: req.ResourceId,
		Metadata:   req.Metadata,
		CreatedAt:  now,
	}

	result, err := s.collection.InsertOne(ctx, event)
	if err != nil {
		log.Printf("Failed to track event: %v", err)
		return nil, err
	}

	oid, ok := result.InsertedID.(primitive.ObjectID)
	if !ok {
		log.Printf("Failed to convert ObjectID")
		return nil, err
	}

	return &pb.TrackEventResponse{
		Event: &pb.Event{
			Id:         oid.Hex(),
			UserId:     event.UserID,
			EventType:  event.EventType,
			ResourceId: event.ResourceID,
			Metadata:   event.Metadata,
			CreatedAt:  event.CreatedAt,
		},
	}, nil
}

func (s *server) GetUserStats(ctx context.Context, req *pb.GetUserStatsRequest) (*pb.GetUserStatsResponse, error) {
	// Parse date range
	startDate := req.StartDate
	endDate := req.EndDate
	if startDate == "" {
		startDate = time.Now().AddDate(0, -1, 0).Format(time.RFC3339) // Default to 1 month ago
	}
	if endDate == "" {
		endDate = time.Now().Format(time.RFC3339) // Default to now
	}

	// Count total tasks
	totalTasksFilter := bson.M{"user_id": req.UserId}
	totalTasks, err := s.taskCollection.CountDocuments(ctx, totalTasksFilter)
	if err != nil {
		return nil, err
	}

	// Count completed tasks
	completedTasksFilter := bson.M{"user_id": req.UserId, "completed": true}
	completedTasks, err := s.taskCollection.CountDocuments(ctx, completedTasksFilter)
	if err != nil {
		return nil, err
	}

	// Count pending tasks
	pendingTasksFilter := bson.M{"user_id": req.UserId, "completed": false}
	pendingTasks, err := s.taskCollection.CountDocuments(ctx, pendingTasksFilter)
	if err != nil {
		return nil, err
	}

	// Count overdue tasks
	now := time.Now().Format(time.RFC3339)
	overdueTasksFilter := bson.M{
		"user_id":   req.UserId,
		"completed": false,
		"due_date":  bson.M{"$lt": now},
	}
	overdueTasks, err := s.taskCollection.CountDocuments(ctx, overdueTasksFilter)
	if err != nil {
		return nil, err
	}

	return &pb.GetUserStatsResponse{
		Stats: &pb.UserStats{
			TotalTasks:     int32(totalTasks),
			CompletedTasks: int32(completedTasks),
			PendingTasks:   int32(pendingTasks),
			OverdueTasks:   int32(overdueTasks),
		},
	}, nil
}

func (s *server) GetTaskStats(ctx context.Context, req *pb.GetTaskStatsRequest) (*pb.GetTaskStatsResponse, error) {
	// Parse date range
	startDate := req.StartDate
	endDate := req.EndDate
	if startDate == "" {
		startDate = time.Now().AddDate(0, -1, 0).Format(time.RFC3339) // Default to 1 month ago
	}
	if endDate == "" {
		endDate = time.Now().Format(time.RFC3339) // Default to now
	}

	// Count total tasks
	totalTasksFilter := bson.M{}
	totalTasks, err := s.taskCollection.CountDocuments(ctx, totalTasksFilter)
	if err != nil {
		return nil, err
	}

	// Count completed tasks
	completedTasksFilter := bson.M{"completed": true}
	completedTasks, err := s.taskCollection.CountDocuments(ctx, completedTasksFilter)
	if err != nil {
		return nil, err
	}

	// Count active users (users with at least one task)
	pipeline := mongo.Pipeline{
		{{"$group", bson.D{{"_id", "$user_id"}}}},
		{{"$count", "count"}},
	}
	cursor, err := s.taskCollection.Aggregate(ctx, pipeline)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var result struct {
		Count int32 `bson:"count"`
	}
	activeUsers := int32(0)
	if cursor.Next(ctx) {
		if err := cursor.Decode(&result); err == nil {
			activeUsers = result.Count
		}
	}

	return &pb.GetTaskStatsResponse{
		Stats: &pb.TaskStats{
			TotalTasks:     int32(totalTasks),
			CompletedTasks: int32(completedTasks),
			ActiveUsers:    activeUsers,
		},
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

	collection := client.Database("todo_app").Collection("events")
	taskCollection := client.Database("todo_app").Collection("tasks")

	// Get port from environment variable
	port := os.Getenv("PORT")
	if port == "" {
		port = "50054"
	}

	lis, err := net.Listen("tcp", ":"+port)
	if err != nil {
		log.Fatalf("Failed to listen: %v", err)
	}

	s := grpc.NewServer()
	pb.RegisterAnalyticsServiceServer(s, &server{
		collection:     collection,
		taskCollection: taskCollection,
	})
	reflection.Register(s)

	log.Printf("Analytics service listening on port %s", port)
	if err := s.Serve(lis); err != nil {
		log.Fatalf("Failed to serve: %v", err)
	}
}
