package auth

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"regexp"
	"strings"
)

var (
	ErrMissingAuth   = errors.New("missing authorization header")
	ErrMissingBearer = errors.New("missing bearer token")
)

type contextKey string

const (
	userRoleKey contextKey = "userRole"
	userIDKey   contextKey = "userId"
)

// GraphQLRequest represents a GraphQL request structure
type GraphQLRequest struct {
	Query         string                 `json:"query"`
	OperationName string                 `json:"operationName"`
	Variables     map[string]interface{} `json:"variables"`
}

// IsIntrospectionQuery checks if the query is a GraphQL introspection query
func IsIntrospectionQuery(query string) bool {
	introspectionPatterns := []string{
		"__schema",
		"__type",
		"IntrospectionQuery",
	}

	// Remove __typename before checking
	query = strings.ReplaceAll(query, "__typename", "")
	query = strings.TrimSpace(query)

	for _, pattern := range introspectionPatterns {
		if strings.Contains(query, pattern) {
			return true
		}
	}
	return false
}

// IsPublicOperation checks if the operation is public
func IsPublicOperation(req *GraphQLRequest) bool {
	publicOperations := map[string]struct{}{
		"login":      {},
		"register":   {},
		"createuser": {},
	}

	operationName := strings.ToLower(req.OperationName)

	// Fallback to extracting operation name from Query
	if operationName == "" && req.Query != "" {
		operationName = extractOperationName(req.Query)
	}

	log.Printf("Determined operation name: %s", operationName)

	if _, isPublic := publicOperations[operationName]; isPublic {
		return true
	}

	return false
}

func extractOperationName(query string) string {
	query = strings.TrimSpace(query)

	// Check if the query starts with `mutation` or `query`
	if strings.HasPrefix(query, "mutation") || strings.HasPrefix(query, "query") {
		parts := strings.Fields(query)
		if len(parts) > 1 {
			// Extract the operation name, even if it's joined with `(`
			operationName := parts[1]
			if idx := strings.Index(operationName, "("); idx != -1 {
				operationName = operationName[:idx] // Remove everything after `(`
			}
			return strings.ToLower(operationName)
		}
	}

	return ""
}

func validateAuthHeader(r *http.Request) (string, error) {
	authHeader := r.Header.Get("Authorization")
	fmt.Printf("Authorization header: %v\n", authHeader)
	if authHeader == "" {
		return "", ErrMissingAuth
	}
	token := strings.TrimPrefix(authHeader, "Bearer ")
	if token == authHeader {
		fmt.Println("Missing 'Bearer' prefix in Authorization header")
		return "", ErrMissingBearer
	}
	if token == "" {
		fmt.Println("Token is empty after trimming 'Bearer' prefix")
		return "", ErrMissingBearer
	}
	return token, nil
}

func AuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Println("AuthMiddleware triggered")
		if r.Method == http.MethodPost {
			bodyBytes, err := io.ReadAll(r.Body)
			if err != nil {
				http.Error(w, `{"errors": [{"message": "invalid request body"}]}`, http.StatusBadRequest)
				return
			}

			r.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))
			var req GraphQLRequest
			if err := json.Unmarshal(bodyBytes, &req); err != nil {
				http.Error(w, `{"errors": [{"message": "invalid request body"}]}`, http.StatusBadRequest)
				return
			}

			// Remove __typename from the query for authentication check
			cleanQuery := removeTypenameFromQuery(req.Query)

			if IsIntrospectionQuery(cleanQuery) || IsPublicOperation(&req) {
				fmt.Println("Skipping auth for public operation")
				next.ServeHTTP(w, r)
				return
			}
		}
		tokenStr, err := validateAuthHeader(r)
		if err != nil {
			http.Error(w, `{"errors": [{"message": "`+err.Error()+`"}]}`, http.StatusUnauthorized)
			return
		}
		claims, err := ValidateJWT(tokenStr)
		if err != nil {
			fmt.Printf("JWT validation error: %v\n", err)
			http.Error(w, `{"errors": [{"message": "`+err.Error()+`"}]}`, http.StatusUnauthorized)
			return
		}
		ctx := context.WithValue(r.Context(), userIDKey, claims.UserID)
		ctx = context.WithValue(ctx, userRoleKey, claims.UserRole)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// Helper function to remove __typename from the query
func removeTypenameFromQuery(query string) string {
	// Simple regex to remove __typename
	re := regexp.MustCompile(`__typename\s*`)
	return re.ReplaceAllString(query, "")
}

// GetAccountType retrieves the account type from context
func GetAccountType(ctx context.Context) (string, bool) {
	userRole, ok := ctx.Value(userRoleKey).(string)
	return userRole, ok
}

// GetUserID retrieves the user ID from context
func GetUserID(ctx context.Context) (string, bool) {
	userID, ok := ctx.Value(userIDKey).(string)
	fmt.Printf("user id: %v\n", userID)
	return userID, ok
}

// RequireAccountType creates a resolver wrapper that checks for required accountType
func RequireAccountType(userRoles ...string) func(ctx context.Context) error {
	return func(ctx context.Context) error {
		userRoleType, ok := GetAccountType(ctx)
		if !ok {
			return errors.New("unauthorized")
		}

		for _, userRole := range userRoles {
			if userRoleType == userRole {
				return nil
			}
		}

		return errors.New("insufficient permissions")
	}
}
