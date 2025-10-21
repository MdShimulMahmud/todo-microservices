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
	pb.UnimplementedTaskServiceServer
	collection *mongo.Collection
}

type Task struct {
	ID          primitive.ObjectID `bson:"_id,omitempty"`
	Title       string             `bson:"title"`
	Description string             `bson:"description"`
	UserID      string             `bson:"user_id"`
	Completed   bool               `bson:"completed"`
	DueDate     string             `bson:"due_date"`
	CreatedAt   string             `bson:"created_at"`
	UpdatedAt   string             `bson:"updated_at"`
}

func (s *server) CreateTask(ctx context.Context, req *pb.CreateTaskRequest) (*pb.TaskResponse, error) {
	now := time.Now().Format(time.RFC3339)
	task := Task{
		Title:       req.Title,
		Description: req.Description,
		UserID:      req.UserId,
		Completed:   false,
		DueDate:     req.DueDate,
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	result, err := s.collection.InsertOne(ctx, task)
	if err != nil {
		log.Printf("Failed to create task: %v", err)
		return nil, err
	}

	oid, ok := result.InsertedID.(primitive.ObjectID)
	if !ok {
		log.Printf("Failed to convert ObjectID")
		return nil, err
	}

	return &pb.TaskResponse{
		Task: &pb.Task{
			Id:          oid.Hex(),
			Title:       task.Title,
			Description: task.Description,
			UserId:      task.UserID,
			Completed:   task.Completed,
			DueDate:     task.DueDate,
			CreatedAt:   task.CreatedAt,
			UpdatedAt:   task.UpdatedAt,
		},
	}, nil
}

func (s *server) GetTask(ctx context.Context, req *pb.GetTaskRequest) (*pb.TaskResponse, error) {
	oid, err := primitive.ObjectIDFromHex(req.Id)
	if err != nil {
		return nil, err
	}

	var task Task
	err = s.collection.FindOne(ctx, bson.M{"_id": oid}).Decode(&task)
	if err != nil {
		return nil, err
	}

	return &pb.TaskResponse{
		Task: &pb.Task{
			Id:          task.ID.Hex(),
			Title:       task.Title,
			Description: task.Description,
			UserId:      task.UserID,
			Completed:   task.Completed,
			DueDate:     task.DueDate,
			CreatedAt:   task.CreatedAt,
			UpdatedAt:   task.UpdatedAt,
		},
	}, nil
}

func (s *server) UpdateTask(ctx context.Context, req *pb.UpdateTaskRequest) (*pb.TaskResponse, error) {
	oid, err := primitive.ObjectIDFromHex(req.Id)
	if err != nil {
		return nil, err
	}

	update := bson.M{
		"$set": bson.M{
			"title":       req.Title,
			"description": req.Description,
			"completed":   req.Completed,
			"due_date":    req.DueDate,
			"updated_at":  time.Now().Format(time.RFC3339),
		},
	}

	_, err = s.collection.UpdateOne(ctx, bson.M{"_id": oid}, update)
	if err != nil {
		return nil, err
	}

	var updatedTask Task
	err = s.collection.FindOne(ctx, bson.M{"_id": oid}).Decode(&updatedTask)
	if err != nil {
		return nil, err
	}

	return &pb.TaskResponse{
		Task: &pb.Task{
			Id:          updatedTask.ID.Hex(),
			Title:       updatedTask.Title,
			Description: updatedTask.Description,
			UserId:      updatedTask.UserID,
			Completed:   updatedTask.Completed,
			DueDate:     updatedTask.DueDate,
			CreatedAt:   updatedTask.CreatedAt,
			UpdatedAt:   updatedTask.UpdatedAt,
		},
	}, nil
}

func (s *server) DeleteTask(ctx context.Context, req *pb.DeleteTaskRequest) (*pb.DeleteTaskResponse, error) {
	oid, err := primitive.ObjectIDFromHex(req.Id)
	if err != nil {
		return nil, err
	}

	_, err = s.collection.DeleteOne(ctx, bson.M{"_id": oid})
	if err != nil {
		return nil, err
	}

	return &pb.DeleteTaskResponse{Success: true}, nil
}

func (s *server) ListTasks(ctx context.Context, req *pb.ListTasksRequest) (*pb.ListTasksResponse, error) {
	filter := bson.M{"user_id": req.UserId}
	if req.Completed {
		filter["completed"] = true
	}

	findOptions := options.Find()
	findOptions.SetLimit(int64(req.Limit))
	findOptions.SetSkip(int64(req.Page * req.Limit))

	cursor, err := s.collection.Find(ctx, filter, findOptions)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var tasks []*pb.Task
	for cursor.Next(ctx) {
		var task Task
		if err := cursor.Decode(&task); err != nil {
			return nil, err
		}
		tasks = append(tasks, &pb.Task{
			Id:          task.ID.Hex(),
			Title:       task.Title,
			Description: task.Description,
			UserId:      task.UserID,
			Completed:   task.Completed,
			DueDate:     task.DueDate,
			CreatedAt:   task.CreatedAt,
			UpdatedAt:   task.UpdatedAt,
		})
	}

	if err := cursor.Err(); err != nil {
		return nil, err
	}

	count, err := s.collection.CountDocuments(ctx, filter)
	if err != nil {
		return nil, err
	}

	return &pb.ListTasksResponse{
		Tasks: tasks,
		Total: int32(count),
	}, nil
}

func main() {
	// Get MongoDB connection string from environment variable
	// Read the environment variables
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

	collection := client.Database("todo_app").Collection("tasks")

	// Get port from environment variable
	port := os.Getenv("PORT")
	if port == "" {
		port = "50051"
	}

	lis, err := net.Listen("tcp", ":"+port)
	if err != nil {
		log.Fatalf("Failed to listen: %v", err)
	}

	s := grpc.NewServer()
	pb.RegisterTaskServiceServer(s, &server{collection: collection})
	reflection.Register(s)

	log.Printf("Task service listening on port %s", port)
	if err := s.Serve(lis); err != nil {
		log.Fatalf("Failed to serve: %v", err)
	}
}
