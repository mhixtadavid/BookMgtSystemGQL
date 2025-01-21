package books

import (
	"bmsgql/auth"
	"bmsgql/database"
	"bmsgql/graph/model"
	"context"
	"fmt"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

func AddBook(ctx context.Context, input model.AddBookInput) (*model.Book, error) {
	BookCollection := database.DB.Collection("Books")

	userID, ok := auth.GetUserID(ctx)
	if !ok || userID == "" {
		fmt.Printf("Error: user ID not found in context or is empty \n")
		return nil, fmt.Errorf("user not authenticated")
	}
	// Validate account type
	accountType, ok := auth.GetAccountType(ctx)
	if !ok || accountType == "" {
		return nil, fmt.Errorf("account type not found in context")
	}

	if accountType != "ADMIN" {
		return nil, fmt.Errorf("access denied: only users with an account type of ADMIN can access this")
	}

	newBook, err := BookCollection.InsertOne(ctx, bson.M{
		"title":       input.Title,
		"author":      input.Author,
		"description": input.Description,
		"category":    input.Category,
		"isbn":        input.Isbn,
		"coverImage":  input.CoverImage,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to add book: %w", err)
	}

	insertedId := newBook.InsertedID.(primitive.ObjectID)

	book := &model.Book{
		ID:          insertedId.Hex(),
		Title:       input.Title,
		Author:      input.Author,
		Description: input.Description,
		Category:    input.Category,
		Isbn:        input.Isbn,
		CoverImage:  input.CoverImage,
	}

	return book, nil
}

// EditBook is the resolver for the editBook field.
func EditBook(ctx context.Context, id string, input model.EditBookInput) (*model.Book, error) {
	BookCollection := database.DB.Collection("Books")

	userID, ok := auth.GetUserID(ctx)
	if !ok || userID == "" {
		fmt.Printf("Error: user ID not found in context or is empty \n")
		return nil, fmt.Errorf("user not authenticated")
	}
	// Validate account type
	accountType, ok := auth.GetAccountType(ctx)
	if !ok || accountType == "" {
		return nil, fmt.Errorf("account type not found in context")
	}

	if accountType != "ADMIN" {
		return nil, fmt.Errorf("access denied: only users with an account type of ADMIN can access this")
	}

	bookId, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return nil, fmt.Errorf("invalid book ID")
	}

	updateBook := bson.M{}

	if input.Title != nil {
		updateBook["title"] = input.Title
	}
	if input.Author != nil {
		updateBook["author"] = input.Author
	}
	if input.Description != nil {
		updateBook["description"] = input.Description
	}
	if input.Category != nil {
		updateBook["category"] = input.Category
	}
	if input.Isbn != nil {
		updateBook["isbn"] = input.Isbn
	}
	if input.CoverImage != nil {
		updateBook["coverImage"] = input.CoverImage
	}

	var book model.Book
	_, err = BookCollection.UpdateOne(ctx, bson.M{"_id": bookId}, bson.M{"$set": updateBook})
	if err != nil {
		return nil, fmt.Errorf("failed to update book: %w", err)
	}

	err = BookCollection.FindOne(ctx, bson.M{"_id": bookId}).Decode(&book)
	if err != nil {
		return nil, fmt.Errorf("book not found")
	}

	return &book, nil
}

// DeleteBook is the resolver for the deleteBook field.
func DeleteBook(ctx context.Context, id string) (bool, error) {
	BookCollection := database.DB.Collection("Books")

	userID, ok := auth.GetUserID(ctx)
	if !ok || userID == "" {
		fmt.Printf("Error: user ID not found in context or is empty \n")
		return false, fmt.Errorf("user not authenticated")
	}
	// Validate account type
	accountType, ok := auth.GetAccountType(ctx)
	if !ok || accountType == "" {
		return false, fmt.Errorf("account type not found in context")
	}

	if accountType != "ADMIN" {
		return false, fmt.Errorf("access denied: only users with an account type of ADMIN can access this")
	}

	bookId, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return false, fmt.Errorf("invalid book ID")
	}
	_, err = BookCollection.DeleteOne(ctx, bson.M{"_id": bookId})
	if err != nil {
		return false, fmt.Errorf("failed to delete book: %w", err)
	}

	return true, nil
}

// BorrowBook is the resolver for the borrowBook field.
func BorrowBook(ctx context.Context, bookID string) (*model.BorrowReceipt, error) {
	panic(fmt.Errorf("not implemented: BorrowBook - borrowBook"))
}

// ReserveBook is the resolver for the reserveBook field.
func ReserveBook(ctx context.Context, bookID string) (*model.ReserveReceipt, error) {
	panic(fmt.Errorf("not implemented: ReserveBook - reserveBook"))
}

func BookDetails(ctx context.Context, id string) (*model.Book, error) {
	BookCollection := database.DB.Collection("Books")

	userID, ok := auth.GetUserID(ctx)
	if !ok || userID == "" {
		fmt.Printf("Error: user ID not found in context or is empty \n")
		return nil, fmt.Errorf("user not authenticated")
	}

	bookId, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return nil, fmt.Errorf("invalid book ID")
	}
	var book model.Book
	err = BookCollection.FindOne(ctx, bson.M{"_id": bookId}).Decode(&book)
	if err != nil {
		return nil, fmt.Errorf("book not found")
	}
	return &book, nil
}

func FeaturedBooks(ctx context.Context) ([]*model.Book, error) {
	BookCollection := database.DB.Collection("Books")

	userID, ok := auth.GetUserID(ctx)
	if !ok || userID == "" {
		fmt.Printf("Error: user ID not found in context or is empty \n")
		return nil, fmt.Errorf("user not authenticated")
	}

	var books []*model.Book
	cursor, err := BookCollection.Find(ctx, bson.M{})
	if err != nil {
		return nil, fmt.Errorf("failed to fetch books: %w", err)
	}
	defer cursor.Close(ctx)
	for cursor.Next(ctx) {
		var book model.Book
		if err := cursor.Decode(&book); err != nil {
			return nil, fmt.Errorf("failed to decode book: %w", err)
		}
		books = append(books, &book)
	}
	if err := cursor.Err(); err != nil {
		return nil, fmt.Errorf("cursor error: %w", err)
	}
	return books, nil
}

// RecentlyViewedBooks is the resolver for the recentlyViewedBooks field.
func RecentlyViewedBooks(ctx context.Context) ([]*model.Book, error) {
	panic(fmt.Errorf("not implemented: RecentlyViewedBooks - recentlyViewedBooks"))
}
