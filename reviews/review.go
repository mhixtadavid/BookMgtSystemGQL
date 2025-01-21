package reviews

import (
	"bmsgql/auth"
	"bmsgql/database"
	"bmsgql/graph/model"
	"context"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

func AddReview(ctx context.Context, bookID string, input model.ReviewInput) (*model.Review, error) {
	BookCollection := database.DB.Collection("Books")
	ReviewCollection := database.DB.Collection("Reviews")
	UserCollection := database.DB.Collection("Users")

	userID, ok := auth.GetUserID(ctx)
	if !ok || userID == "" {
		fmt.Printf("Error: user ID not found in context or is empty \n")
		return nil, fmt.Errorf("user not authenticated")
	}

	userObjId, err := primitive.ObjectIDFromHex(userID)
	if err != nil {
		return nil, fmt.Errorf("invalid user ID")
	}

	bookId, err := primitive.ObjectIDFromHex(bookID)
	if err != nil {
		return nil, fmt.Errorf("invalid book ID")
	}

	var user model.User
	err = UserCollection.FindOne(ctx, bson.M{"_id": userObjId}).Decode(&user)
	if err != nil {
		return nil, fmt.Errorf("user not found")
	}

	var book model.Book
	err = BookCollection.FindOne(ctx, bson.M{"_id": bookId}).Decode(&book)
	if err != nil {
		return nil, fmt.Errorf("book not found")
	}

	newReview, err := ReviewCollection.InsertOne(ctx, bson.M{
		"bookId":    bookId,
		"userId":    userObjId,
		"rating":    input.Rating,
		"content":   input.Content,
		"createdAt": time.Now().Format(time.RFC3339),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to add review: %w", err)
	}

	insertedId := newReview.InsertedID.(primitive.ObjectID)

	review := &model.Review{
		ID:      insertedId.Hex(),
		Book:    &book,
		User:    &user,
		Rating:  input.Rating,
		Content: input.Content,
	}

	// update the book's rating
	if book.Rating == 0 {
		book.Rating = input.Rating
	} else {
		book.Rating = (book.Rating + input.Rating) / 2
	}

	_, err = BookCollection.UpdateOne(ctx, bson.M{"_id": bookId}, bson.M{"$addToSet": bson.M{"reviews": insertedId}})
	if err != nil {
		return nil, fmt.Errorf("failed to update book: %w", err)
	}
	return review, nil
}

// func EditReview(ctx context.Context, reviewID string, input model.ReviewInput) (*model.Review, error) {
// 	BookCollection := database.DB.Collection("Books")
// 	ReviewCollection := database.DB.Collection("Reviews")
// 	UserCollection := database.DB.Collection("Users")

// 	userID, ok := auth.GetUserID(ctx)
// 	if !ok || userID == "" {
// 		fmt.Printf("Error: user ID not found in context or is empty \n")
// 		return nil, fmt.Errorf("user not authenticated")
// 	}

// 	userObjId, err := primitive.ObjectIDFromHex(userID)
// 	if err != nil {
// 		return nil, fmt.Errorf("invalid user ID")
// 	}

// 	bookId, err := primitive.ObjectIDFromHex(bookID)
// 	if err != nil {
// 		return nil, fmt.Errorf("invalid book ID")
// 	}

// 	var user model.User
// 	err = UserCollection.FindOne(ctx, bson.M{"_id": userObjId}).Decode(&user)
// 	if err != nil {
// 		return nil, fmt.Errorf("user not found")
// 	}

// 	var book model.Book
// 	err = BookCollection.FindOne(ctx, bson.M{"_id": bookId}).Decode(&book)
// 	if err != nil {
// 		return nil, fmt.Errorf("book not found")
// 	}
// }

func DeleteReview(ctx context.Context, reviewID string) (bool, error) {
	panic(fmt.Errorf("not implemented: DeleteReview - deleteReview"))
}

func BookReviews(ctx context.Context, bookID string) ([]*model.Review, error) {
	BookCollection := database.DB.Collection("Books")
	ReviewCollection := database.DB.Collection("Reviews")

	userID, ok := auth.GetUserID(ctx)
	if !ok || userID == "" {
		fmt.Printf("Error: user ID not found in context or is empty \n")
		return nil, fmt.Errorf("user not authenticated")
	}

	bookId, err := primitive.ObjectIDFromHex(bookID)
	if err != nil {
		return nil, fmt.Errorf("invalid book ID")
	}

	var book model.Book
	err = BookCollection.FindOne(ctx, bson.M{"_id": bookId}).Decode(&book)
	if err != nil {
		return nil, fmt.Errorf("book not found")
	}

	var reviews []*model.Review
	cursor, err := ReviewCollection.Find(ctx, bson.M{"bookId": bookId})
	if err != nil {
		return nil, fmt.Errorf("failed to fetch reviews: %w", err)
	}
	defer cursor.Close(ctx)
	for cursor.Next(ctx) {
		var review model.Review
		if err := cursor.Decode(&review); err != nil {
			return nil, fmt.Errorf("failed to decode review: %w", err)
		}
		reviews = append(reviews, &review)
	}
	if err := cursor.Err(); err != nil {
		return nil, fmt.Errorf("cursor error: %w", err)
	}
	return reviews, nil
}
