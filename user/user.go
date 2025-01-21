package user

import (
	"bmsgql/auth"
	"bmsgql/database"
	"bmsgql/graph/model"
	"context"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"golang.org/x/crypto/bcrypt"
)

func getUserByEmail(ctx context.Context, email string) (*model.User, error) {
	UserCollection := database.DB.Collection("Users")

	var user model.User
	err := UserCollection.FindOne(ctx, bson.M{"email": email}).Decode(&user)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to find user by email: %v", err)
	}
	return &user, nil
}

func Login(email string, password string) (*model.AuthPayload, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	user, err := getUserByEmail(ctx, email)
	if err != nil {
		return nil, err
	}
	if user == nil {
		return nil, fmt.Errorf("user not found")
	}

	err = bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(password))
	if err != nil {
		return nil, fmt.Errorf("invalid password")
	}

	// generate JWT token

	token, err := auth.GenerateJWT(user.ID, string(user.Role))
	if err != nil {
		return nil, fmt.Errorf("error generating jwt token")
	}

	return &model.AuthPayload{
		Token: token,
		User:  user,
	}, nil
}

func SignUp(input model.SignUpInput) (*model.AuthPayload, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	UserCollection := database.DB.Collection("Users")

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(input.Password), bcrypt.DefaultCost)
	if err != nil {
		return nil, fmt.Errorf("failed to hash password: %v", err)
	}

	newUser, err := UserCollection.InsertOne(ctx, bson.M{
		"name":           input.Name,
		"email":          input.Email,
		"password":       string(hashedPassword),
		"role":           model.UserRoleReader,
		"favoriteGenres": input.FavoriteGenres,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create user: %v", err)
	}

	insertedId := newUser.InsertedID.(primitive.ObjectID)

	user := &model.User{
		ID:             insertedId.Hex(),
		Name:           input.Name,
		Email:          input.Email,
		Password:       string(hashedPassword),
		Role:           model.UserRoleReader,
		FavoriteGenres: input.FavoriteGenres,
	}

	// Generate JWT token
	token, err := auth.GenerateJWT(user.ID, string(user.Role))
	if err != nil {
		return nil, fmt.Errorf("error generating jwt token")
	}

	// Return AuthPayload
	return &model.AuthPayload{
		Token: token,
		User:  user,
	}, nil
}

func RecoverPassword(ctx context.Context, email string) (bool, error) {
	panic(fmt.Errorf("not implemented: RecoverPassword - recoverPassword"))
}

func ResetPassword(ctx context.Context, otp string, newPassword string) (bool, error) {
	panic(fmt.Errorf("not implemented: ResetPassword - resetPassword"))
}

func CurrentUser(ctx context.Context) (*model.User, error) {
	UserCollection := database.DB.Collection("Users")

	userID, ok := auth.GetUserID(ctx)
	if !ok || userID == "" {
		fmt.Printf("Error: user ID not found in context or is empty \n")
		return nil, fmt.Errorf("user not authenticated")
	}
	fmt.Printf("UserID retrieved from context: %s\n", userID)

	userObjID, err := primitive.ObjectIDFromHex(userID)
	if err != nil {
		return nil, fmt.Errorf("invalid user id: %v", err)
	}

	var user model.User
	err = UserCollection.FindOne(ctx, bson.M{"_id": userObjID}).Decode(&user)
	if err != nil {
		return nil, fmt.Errorf("error finding user: %v", err)
	}
	return &user, nil
}
