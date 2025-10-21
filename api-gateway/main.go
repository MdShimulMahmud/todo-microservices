package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/gorilla/handlers"
	"github.com/gorilla/mux"
	"github.com/gorilla/schema"
	pb "github.com/technonext/todo-app/proto/proto"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// Service clients
type ServiceClients struct {
	taskClient         pb.TaskServiceClient
	userClient         pb.UserServiceClient
	notificationClient pb.NotificationServiceClient
	analyticsClient    pb.AnalyticsServiceClient
}

var decoder = schema.NewDecoder()

func main() {
	// Initialize service connections
	clients := initServiceClients()

	// Create router
	router := mux.NewRouter()

	// Health check
	router.HandleFunc("/health", healthCheckHandler).Methods("GET")

	// Task routes
	router.HandleFunc("/api/tasks", createTaskHandler(clients)).Methods("POST")
	router.HandleFunc("/api/tasks/{id}", getTaskHandler(clients)).Methods("GET")
	router.HandleFunc("/api/tasks/{id}", updateTaskHandler(clients)).Methods("PUT")
	router.HandleFunc("/api/tasks/{id}", deleteTaskHandler(clients)).Methods("DELETE")
	router.HandleFunc("/api/tasks", listTasksHandler(clients)).Methods("GET")

	// User routes
	router.HandleFunc("/api/users", createUserHandler(clients)).Methods("POST")
	router.HandleFunc("/api/users/{id}", getUserHandler(clients)).Methods("GET")
	router.HandleFunc("/api/users/{id}", updateUserHandler(clients)).Methods("PUT")
	router.HandleFunc("/api/users/{id}", deleteUserHandler(clients)).Methods("DELETE")
	router.HandleFunc("/api/auth", authHandler(clients)).Methods("POST")

	// Notification routes
	router.HandleFunc("/api/notifications", sendNotificationHandler(clients)).Methods("POST")
	router.HandleFunc("/api/notifications", getNotificationsHandler(clients)).Methods("GET")

	// Analytics routes
	router.HandleFunc("/api/analytics/events", trackEventHandler(clients)).Methods("POST")
	router.HandleFunc("/api/analytics/users/{id}/stats", getUserStatsHandler(clients)).Methods("GET")
	router.HandleFunc("/api/analytics/tasks/stats", getTaskStatsHandler(clients)).Methods("GET")

	// CORS handler
	corsHandler := handlers.CORS(
		handlers.AllowedOrigins([]string{"*"}),
		handlers.AllowedMethods([]string{"GET", "POST", "PUT", "DELETE", "OPTIONS"}),
		handlers.AllowedHeaders([]string{"Content-Type", "Authorization"}),
	)

	// Start server
	port := getEnv("PORT", "8080")
	log.Printf("API Gateway starting on port %s", port)
	log.Fatal(http.ListenAndServe(":"+port, corsHandler(router)))
}

func initServiceClients() *ServiceClients {
	// Get service addresses from environment variables
	taskServiceAddr := getEnv("TASK_SERVICE_ADDR", "localhost:50051")
	userServiceAddr := getEnv("USER_SERVICE_ADDR", "localhost:50052")
	notificationServiceAddr := getEnv("NOTIFICATION_SERVICE_ADDR", "localhost:50053")
	analyticsServiceAddr := getEnv("ANALYTICS_SERVICE_ADDR", "localhost:50054")

	// Set up connections to services
	taskConn, err := grpc.Dial(taskServiceAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("Failed to connect to task service: %v", err)
	}

	userConn, err := grpc.Dial(userServiceAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("Failed to connect to user service: %v", err)
	}

	notificationConn, err := grpc.Dial(notificationServiceAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("Failed to connect to notification service: %v", err)
	}

	analyticsConn, err := grpc.Dial(analyticsServiceAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("Failed to connect to analytics service: %v", err)
	}

	return &ServiceClients{
		taskClient:         pb.NewTaskServiceClient(taskConn),
		userClient:         pb.NewUserServiceClient(userConn),
		notificationClient: pb.NewNotificationServiceClient(notificationConn),
		analyticsClient:    pb.NewAnalyticsServiceClient(analyticsConn),
	}
}

// Helper functions
func getEnv(key, fallback string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return fallback
}

func respondWithJSON(w http.ResponseWriter, code int, payload interface{}) {
	response, _ := json.Marshal(payload)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	w.Write(response)
}

func respondWithError(w http.ResponseWriter, code int, message string) {
	respondWithJSON(w, code, map[string]string{"error": message})
}

// Health check handler
func healthCheckHandler(w http.ResponseWriter, r *http.Request) {
	respondWithJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// Task handlers
func createTaskHandler(clients *ServiceClients) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if clients == nil || clients.taskClient == nil {
			respondWithError(w, http.StatusServiceUnavailable, "task service unavailable")
			return
		}
		var req pb.CreateTaskRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			respondWithError(w, http.StatusBadRequest, "Invalid request payload")
			return
		}

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		resp, err := clients.taskClient.CreateTask(ctx, &req)
		if err != nil {
			respondWithError(w, http.StatusInternalServerError, err.Error())
			return
		}

		respondWithJSON(w, http.StatusCreated, resp)
	}
}

func getTaskHandler(clients *ServiceClients) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if clients == nil || clients.taskClient == nil {
			respondWithError(w, http.StatusServiceUnavailable, "task service unavailable")
			return
		}
		vars := mux.Vars(r)
		id := vars["id"]

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		resp, err := clients.taskClient.GetTask(ctx, &pb.GetTaskRequest{Id: id})
		if err != nil {
			respondWithError(w, http.StatusInternalServerError, err.Error())
			return
		}

		respondWithJSON(w, http.StatusOK, resp)
	}
}

func updateTaskHandler(clients *ServiceClients) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if clients == nil || clients.taskClient == nil {
			respondWithError(w, http.StatusServiceUnavailable, "task service unavailable")
			return
		}
		vars := mux.Vars(r)
		id := vars["id"]

		var req pb.UpdateTaskRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			respondWithError(w, http.StatusBadRequest, "Invalid request payload")
			return
		}
		req.Id = id

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		resp, err := clients.taskClient.UpdateTask(ctx, &req)
		if err != nil {
			respondWithError(w, http.StatusInternalServerError, err.Error())
			return
		}

		respondWithJSON(w, http.StatusOK, resp)
	}
}

func deleteTaskHandler(clients *ServiceClients) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if clients == nil || clients.taskClient == nil {
			respondWithError(w, http.StatusServiceUnavailable, "task service unavailable")
			return
		}
		vars := mux.Vars(r)
		id := vars["id"]

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		resp, err := clients.taskClient.DeleteTask(ctx, &pb.DeleteTaskRequest{Id: id})
		if err != nil {
			respondWithError(w, http.StatusInternalServerError, err.Error())
			return
		}

		respondWithJSON(w, http.StatusOK, resp)
	}
}

func listTasksHandler(clients *ServiceClients) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if clients == nil || clients.taskClient == nil {
			respondWithError(w, http.StatusServiceUnavailable, "task service unavailable")
			return
		}
		var req pb.ListTasksRequest
		if err := decoder.Decode(&req, r.URL.Query()); err != nil {
			respondWithError(w, http.StatusBadRequest, "Invalid query parameters")
			return
		}

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		resp, err := clients.taskClient.ListTasks(ctx, &req)
		if err != nil {
			respondWithError(w, http.StatusInternalServerError, err.Error())
			return
		}

		respondWithJSON(w, http.StatusOK, resp)
	}
}

// User handlers
func createUserHandler(clients *ServiceClients) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if clients == nil || clients.userClient == nil {
			respondWithError(w, http.StatusServiceUnavailable, "user service unavailable")
			return
		}
		var req pb.CreateUserRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			respondWithError(w, http.StatusBadRequest, "Invalid request payload")
			return
		}

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		resp, err := clients.userClient.CreateUser(ctx, &req)
		if err != nil {
			respondWithError(w, http.StatusInternalServerError, err.Error())
			return
		}

		respondWithJSON(w, http.StatusCreated, resp)
	}
}

func getUserHandler(clients *ServiceClients) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if clients == nil || clients.userClient == nil {
			respondWithError(w, http.StatusServiceUnavailable, "user service unavailable")
			return
		}
		vars := mux.Vars(r)
		id := vars["id"]

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		resp, err := clients.userClient.GetUser(ctx, &pb.GetUserRequest{Id: id})
		if err != nil {
			respondWithError(w, http.StatusInternalServerError, err.Error())
			return
		}

		respondWithJSON(w, http.StatusOK, resp)
	}
}

func updateUserHandler(clients *ServiceClients) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if clients == nil || clients.userClient == nil {
			respondWithError(w, http.StatusServiceUnavailable, "user service unavailable")
			return
		}
		vars := mux.Vars(r)
		id := vars["id"]

		var req pb.UpdateUserRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			respondWithError(w, http.StatusBadRequest, "Invalid request payload")
			return
		}
		req.Id = id

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		resp, err := clients.userClient.UpdateUser(ctx, &req)
		if err != nil {
			respondWithError(w, http.StatusInternalServerError, err.Error())
			return
		}

		respondWithJSON(w, http.StatusOK, resp)
	}
}

func deleteUserHandler(clients *ServiceClients) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if clients == nil || clients.userClient == nil {
			respondWithError(w, http.StatusServiceUnavailable, "user service unavailable")
			return
		}
		vars := mux.Vars(r)
		id := vars["id"]

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		resp, err := clients.userClient.DeleteUser(ctx, &pb.DeleteUserRequest{Id: id})
		if err != nil {
			respondWithError(w, http.StatusInternalServerError, err.Error())
			return
		}

		respondWithJSON(w, http.StatusOK, resp)
	}
}

func authHandler(clients *ServiceClients) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if clients == nil || clients.userClient == nil {
			respondWithError(w, http.StatusServiceUnavailable, "user service unavailable")
			return
		}
		var req pb.AuthRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			respondWithError(w, http.StatusBadRequest, "Invalid request payload")
			return
		}

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		resp, err := clients.userClient.AuthenticateUser(ctx, &req)
		if err != nil {
			respondWithError(w, http.StatusUnauthorized, "Authentication failed")
			return
		}

		respondWithJSON(w, http.StatusOK, resp)
	}
}

// Notification handlers
func sendNotificationHandler(clients *ServiceClients) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if clients == nil || clients.notificationClient == nil {
			respondWithError(w, http.StatusServiceUnavailable, "notification service unavailable")
			return
		}
		var req pb.NotificationRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			respondWithError(w, http.StatusBadRequest, "Invalid request payload")
			return
		}

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		resp, err := clients.notificationClient.SendNotification(ctx, &req)
		if err != nil {
			respondWithError(w, http.StatusInternalServerError, err.Error())
			return
		}

		respondWithJSON(w, http.StatusCreated, resp)
	}
}

func getNotificationsHandler(clients *ServiceClients) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if clients == nil || clients.notificationClient == nil {
			respondWithError(w, http.StatusServiceUnavailable, "notification service unavailable")
			return
		}
		var req pb.GetNotificationsRequest
		if err := decoder.Decode(&req, r.URL.Query()); err != nil {
			respondWithError(w, http.StatusBadRequest, "Invalid query parameters")
			return
		}

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		resp, err := clients.notificationClient.GetNotifications(ctx, &req)
		if err != nil {
			respondWithError(w, http.StatusInternalServerError, err.Error())
			return
		}

		respondWithJSON(w, http.StatusOK, resp)
	}
}

// Analytics handlers
func trackEventHandler(clients *ServiceClients) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if clients == nil || clients.analyticsClient == nil {
			respondWithError(w, http.StatusServiceUnavailable, "analytics service unavailable")
			return
		}
		var req pb.TrackEventRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			respondWithError(w, http.StatusBadRequest, "Invalid request payload")
			return
		}

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		resp, err := clients.analyticsClient.TrackEvent(ctx, &req)
		if err != nil {
			respondWithError(w, http.StatusInternalServerError, err.Error())
			return
		}

		respondWithJSON(w, http.StatusCreated, resp)
	}
}

func getUserStatsHandler(clients *ServiceClients) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if clients == nil || clients.analyticsClient == nil {
			respondWithError(w, http.StatusServiceUnavailable, "analytics service unavailable")
			return
		}
		vars := mux.Vars(r)
		userId := vars["id"]

		var req pb.GetUserStatsRequest
		if err := decoder.Decode(&req, r.URL.Query()); err != nil {
			respondWithError(w, http.StatusBadRequest, "Invalid query parameters")
			return
		}
		req.UserId = userId

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		resp, err := clients.analyticsClient.GetUserStats(ctx, &req)
		if err != nil {
			respondWithError(w, http.StatusInternalServerError, err.Error())
			return
		}

		respondWithJSON(w, http.StatusOK, resp)
	}
}

func getTaskStatsHandler(clients *ServiceClients) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if clients == nil || clients.analyticsClient == nil {
			respondWithError(w, http.StatusServiceUnavailable, "analytics service unavailable")
			return
		}
		var req pb.GetTaskStatsRequest
		if err := decoder.Decode(&req, r.URL.Query()); err != nil {
			respondWithError(w, http.StatusBadRequest, "Invalid query parameters")
			return
		}

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		resp, err := clients.analyticsClient.GetTaskStats(ctx, &req)
		if err != nil {
			respondWithError(w, http.StatusInternalServerError, err.Error())
			return
		}

		respondWithJSON(w, http.StatusOK, resp)
	}
}
