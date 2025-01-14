package database

import (
	"bmsgql/graph/model"
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.mongodb.org/mongo-driver/mongo/readpref"
	"golang.org/x/crypto/bcrypt"
)

var DB *mongo.Database

func Connect() (*mongo.Client, error) {
	mongoURI := os.Getenv("MONGO_DB_URI")
	if mongoURI == "" {
		return nil, fmt.Errorf("MONGO_DB_URI is not set")
	}

	databaseName := os.Getenv("DATABASE")
	if databaseName == "" {
		return nil, fmt.Errorf("DATABASE name is not set")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	client, err := mongo.Connect(ctx, options.Client().ApplyURI(mongoURI))
	if err != nil {
		return nil, fmt.Errorf("failed to connect to MongoDB: %w", err)
	}

	if err := client.Ping(ctx, readpref.Primary()); err != nil {
		return nil, fmt.Errorf("failed to ping MongoDB: %w", err)
	}

	DB = client.Database(databaseName)
	fmt.Println("Successfully connected to the database")
	return client, nil
}

func GetBooks() []*model.Book {
	bookCollection := DB.Collection("Books")
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	var bookListings []*model.Book
	cur, err := bookCollection.Find(ctx, bson.D{})
	if err != nil {
		fmt.Println("Error:", err)
	}
	defer cur.Close(ctx)
	if err = cur.All(ctx, &bookListings); err != nil {
		fmt.Println("Error:", err)
	}

	return bookListings
}

func GetBook(id string) *model.Book {
	bookCollection := DB.Collection("Books")
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	_id, _ := primitive.ObjectIDFromHex(id)
	filter := bson.M{"_id": _id}

	var bookListing model.Book
	err := bookCollection.FindOne(ctx, filter).Decode(&bookListing)
	if err != nil {
		fmt.Println("Error:", err)
	}
	return &bookListing
}

func CreateBookInput(BookInfo model.BookInput) *model.Book {
	authorCollections := DB.Collection("Authors")
	publisherCollections := DB.Collection("Publishers")
	bookCollection := DB.Collection("Books")
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Convert author IDs to ObjectIDs for MongoDB references
	var authorObjectIDs []primitive.ObjectID
	for _, authorID := range BookInfo.AuthorIds {
		objectID, err := primitive.ObjectIDFromHex(authorID)
		if err != nil {
			fmt.Println("Error:", err)
		}
		authorObjectIDs = append(authorObjectIDs, objectID)
	}

	// Convert the publisher ID, if available
	var publisherObjectID primitive.ObjectID
	if BookInfo.PublisherID != nil {
		var err error
		publisherObjectID, err = primitive.ObjectIDFromHex(*BookInfo.PublisherID)
		if err != nil {
			fmt.Println("Error:", err)
		}
	}

	// Create the book document
	bookDoc := bson.M{
		"title":              BookInfo.Title,
		"isbn":               BookInfo.Isbn,
		"description":        BookInfo.Description,
		"publishedYear":      BookInfo.PublishedYear,
		"pageCount":          BookInfo.PageCount,
		"language":           BookInfo.Language,
		"category":           BookInfo.Category,
		"authors":            authorObjectIDs,
		"price":              BookInfo.Price,
		"discountPercentage": BookInfo.DiscountPercentage,
		"totalCopies":        BookInfo.TotalCopies,
		"coverImageURL":      BookInfo.CoverImageURL,
		"tags":               BookInfo.Tags,
	}

	// Add publisher reference only if it's valid
	if BookInfo.PublisherID != nil {
		bookDoc["publisher"] = publisherObjectID
	}

	// Insert the book into the database
	inserted, err := bookCollection.InsertOne(ctx, bookDoc)
	if err != nil {
		fmt.Println("Error:", err)
	}

	// Retrieve and populate author details
	var authors []*model.Author
	for _, authorID := range authorObjectIDs {
		var author model.Author
		err := authorCollections.FindOne(ctx, bson.M{"_id": authorID}).Decode(&author)
		if err != nil {
			fmt.Println("Error:", err)
		}
		authors = append(authors, &author)
	}

	// Retrieve and populate publisher details, if applicable
	var publisher *model.Publisher
	if BookInfo.PublisherID != nil {
		var pub model.Publisher
		err := publisherCollections.FindOne(ctx, bson.M{"_id": publisherObjectID}).Decode(&pub)
		if err != nil {
			fmt.Println("Error:", err)
		}
		publisher = &pub
	}

	insertedID := inserted.InsertedID.(primitive.ObjectID).Hex()
	returnBook := model.Book{
		ID:                 insertedID,
		Title:              BookInfo.Title,
		Isbn:               BookInfo.Isbn,
		Description:        BookInfo.Description,
		PublishedYear:      BookInfo.PublishedYear,
		PageCount:          BookInfo.PageCount,
		Language:           BookInfo.Language,
		Category:           BookInfo.Category,
		Authors:            authors,
		Publisher:          publisher, // Using the properly created publisher reference
		Price:              BookInfo.Price,
		DiscountPercentage: BookInfo.DiscountPercentage,
		TotalCopies:        BookInfo.TotalCopies,
		CoverImageURL:      BookInfo.CoverImageURL,
		Tags:               BookInfo.Tags,
		Status:             nil,
		AvailableCopies:    nil,
		AverageRating:      nil,
		TotalRatings:       nil,
	}

	return &returnBook
}

func UpdateBook(BookId string, BookInfo model.BookUpdateInput) *model.Book {
	bookCollection := DB.Collection("Books")
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	updateBookInfo := bson.M{}

	if BookInfo.Title != nil {
		updateBookInfo["title"] = BookInfo.Title
	}
	if BookInfo.Isbn != nil {
		updateBookInfo["isbn"] = BookInfo.Isbn
	}
	if BookInfo.Description != nil {
		updateBookInfo["description"] = BookInfo.Description
	}
	if BookInfo.PublishedYear != nil {
		updateBookInfo["publishedYear"] = BookInfo.PublishedYear
	}
	if BookInfo.PageCount != nil {
		updateBookInfo["pageCount"] = BookInfo.PageCount
	}
	if BookInfo.Language != nil {
		updateBookInfo["language"] = BookInfo.Language
	}
	if BookInfo.Category != nil {
		updateBookInfo["category"] = BookInfo.Category
	}

	// Pricing and inventory
	if BookInfo.Price != nil {
		updateBookInfo["price"] = BookInfo.Price
	}
	if BookInfo.DiscountPercentage != nil {
		updateBookInfo["discountPercentage"] = BookInfo.DiscountPercentage
	}
	if BookInfo.TotalCopies != nil {
		updateBookInfo["totalCopies"] = BookInfo.TotalCopies
	}

	// Additional metadata
	if BookInfo.CoverImageURL != nil {
		updateBookInfo["coverImageURL"] = BookInfo.CoverImageURL
	}
	if BookInfo.Tags != nil {
		updateBookInfo["tags"] = BookInfo.Tags
	}

	_id, err := primitive.ObjectIDFromHex(BookId)
	if err != nil {
		fmt.Println("Error:", err)
	}
	filter := bson.M{"_id": _id}
	update := bson.M{"$set": updateBookInfo}

	results := bookCollection.FindOneAndUpdate(ctx, filter, update, options.FindOneAndUpdate().SetReturnDocument(1))

	var bookListing model.Book
	if err = results.Decode(&bookListing); err != nil {
		fmt.Println("Error:", err)
	}
	return &bookListing
}

func DeleteBook(id string) error {
	bookCollection := DB.Collection("Books")
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	_id, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		fmt.Println("Error:", err)
	}
	filter := bson.M{"_id": _id}

	var bookListing model.Book
	err = bookCollection.FindOne(ctx, filter).Decode(&bookListing)
	if err != nil {
		fmt.Println("Error:", err)
	}

	_, err = bookCollection.DeleteOne(ctx, filter)
	if err != nil {
		fmt.Println("Error:", err)
	}

	return nil
}

func GetAuthours() []*model.Author {
	authorCollections := DB.Collection("Authors")
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	var authorListings []*model.Author
	cur, err := authorCollections.Find(ctx, bson.D{})
	if err != nil {
		fmt.Println("Error:", err)
	}
	defer cur.Close(ctx)
	if err = cur.All(ctx, &authorListings); err != nil {
		fmt.Println("Error:", err)
	}

	return authorListings
}

func GetAuthour(id string) *model.Author {
	authorCollections := DB.Collection("Authors")
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	_id, _ := primitive.ObjectIDFromHex(id)
	filter := bson.M{"_id": _id}

	var authorListing model.Author
	err := authorCollections.FindOne(ctx, filter).Decode(&authorListing)
	if err != nil {
		fmt.Println("Error:", err)
	}
	return &authorListing
}

func CreateAuthorInput(AuthorInfo model.AuthorInput) *model.Author {
	authorCollections := DB.Collection("Authors")
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Insert author using a bson.M map
	inserted, err := authorCollections.InsertOne(ctx, bson.M{
		"name":        AuthorInfo.Name,
		"biography":   AuthorInfo.Biography,
		"birthDate":   AuthorInfo.BirthDate,
		"nationality": AuthorInfo.Nationality,
		"awards":      AuthorInfo.Awards,
		"websiteURL":  AuthorInfo.WebsiteURL,
	})
	if err != nil {
		log.Fatal(err)
	}

	// Convert the InsertedID to a hex string
	insertedID := inserted.InsertedID.(primitive.ObjectID).Hex()

	// Populate the return model.Author struct with the hex ID and other details
	returnAuthor := model.Author{
		ID:          insertedID,
		Name:        AuthorInfo.Name,
		Biography:   AuthorInfo.Biography,
		BirthDate:   AuthorInfo.BirthDate,
		Nationality: AuthorInfo.Nationality,
		Awards:      AuthorInfo.Awards,
		WebsiteURL:  AuthorInfo.WebsiteURL,
	}

	return &returnAuthor
}

func UpdateAuthor(AuthorId string, AuthorInfo model.AuthorUpdateInput) *model.Author {
	authorCollections := DB.Collection("Authors")
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	updateAuthorInfo := bson.M{}

	if AuthorInfo.Name != nil {
		updateAuthorInfo["name"] = AuthorInfo.Name
	}
	if AuthorInfo.Biography != nil {
		updateAuthorInfo["biography"] = AuthorInfo.Biography
	}
	if AuthorInfo.BirthDate != nil {
		updateAuthorInfo["birthDate"] = AuthorInfo.BirthDate
	}
	if AuthorInfo.Nationality != nil {
		updateAuthorInfo["nationality"] = AuthorInfo.Nationality
	}
	if len(AuthorInfo.Awards) > 0 {
		updateAuthorInfo["awards"] = AuthorInfo.Awards
	}
	if AuthorInfo.WebsiteURL != nil {
		updateAuthorInfo["websiteURL"] = AuthorInfo.WebsiteURL
	}
	_id, err := primitive.ObjectIDFromHex(AuthorId)
	if err != nil {
		fmt.Println("Error:", err)
	}
	filter := bson.M{"_id": _id}
	update := bson.M{"$set": updateAuthorInfo}

	results := authorCollections.FindOneAndUpdate(ctx, filter, update, options.FindOneAndUpdate().SetReturnDocument(1))

	var authorListing model.Author
	if err = results.Decode(&authorListing); err != nil {
		fmt.Println("Error:", err)
	}
	return &authorListing
}

func DeleteAuthor(id string) *model.Author {
	authorCollections := DB.Collection("Authors")
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	_id, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		fmt.Println("Error:", err)
	}
	filter := bson.M{"_id": _id}

	var authorListing model.Author
	err = authorCollections.FindOne(ctx, filter).Decode(&authorListing)
	if err != nil {
		fmt.Println("Error:", err)
	}
	_, err = authorCollections.DeleteOne(ctx, filter)
	if err != nil {
		fmt.Println("Error:", err)
	}
	return &authorListing
}

func GetPublishers() []*model.Publisher {
	publisherCollections := DB.Collection("Publishers")
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	var publisherListings []*model.Publisher
	cur, err := publisherCollections.Find(ctx, bson.D{})
	if err != nil {
		fmt.Println("Error:", err)
	}
	defer cur.Close(ctx)
	if err = cur.All(ctx, &publisherListings); err != nil {
		fmt.Println("Error:", err)
	}

	return publisherListings
}

func GetPublisher(id string) *model.Publisher {
	publisherCollections := DB.Collection("Publishers")
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	_id, _ := primitive.ObjectIDFromHex(id)
	filter := bson.M{"_id": _id}

	var publisherListing model.Publisher
	err := publisherCollections.FindOne(ctx, filter).Decode(&publisherListing)
	if err != nil {
		fmt.Println("Error:", err)
	}

	return &publisherListing
}

func CreatePublisherInput(PublisherInfo model.PublisherInput) *model.Publisher {
	publisherCollections := DB.Collection("Publishers")
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Insert publisher using a bson.M map
	inserted, err := publisherCollections.InsertOne(ctx, bson.M{
		"name":        PublisherInfo.Name,
		"foundedYear": PublisherInfo.FoundedYear,
		"location":    PublisherInfo.Location,
		"websiteURL":  PublisherInfo.WebsiteURL,
	})
	if err != nil {
		log.Fatal(err)
	}

	// Convert the InsertedID to a hex string
	insertedID := inserted.InsertedID.(primitive.ObjectID).Hex()

	// Populate the return model.Publisher struct with the hex ID and other details
	returnPublisher := model.Publisher{
		ID:          insertedID,
		Name:        PublisherInfo.Name,
		FoundedYear: PublisherInfo.FoundedYear,
		Location:    PublisherInfo.Location,
		WebsiteURL:  PublisherInfo.WebsiteURL,
	}

	return &returnPublisher
}

func UpdatePublisher(Publisherid string, PublisherInfo model.PublisherUpdateInput) *model.Publisher {
	publisherCollections := DB.Collection("Publishers")
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	updatePublisherInfo := bson.M{}

	if PublisherInfo.Name != nil {
		updatePublisherInfo["name"] = PublisherInfo.Name
	}
	if PublisherInfo.FoundedYear != nil {
		updatePublisherInfo["foundedYear"] = PublisherInfo.FoundedYear
	}
	if PublisherInfo.Location != nil {
		updatePublisherInfo["location"] = PublisherInfo.Location
	}
	if PublisherInfo.WebsiteURL != nil {
		updatePublisherInfo["websiteURL"] = PublisherInfo.WebsiteURL
	}

	_id, err := primitive.ObjectIDFromHex(Publisherid)
	if err != nil {
		fmt.Println("Error:", err)
	}
	filter := bson.M{"_id": _id}
	update := bson.M{"$set": updatePublisherInfo}

	results := publisherCollections.FindOneAndUpdate(ctx, filter, update, options.FindOneAndUpdate().SetReturnDocument(1))

	var publisherListing model.Publisher
	err = results.Decode(&publisherListing)
	if err != nil {
		log.Panic(err)
	}
	return &publisherListing
}

func DeletePublisher(id string) *model.Publisher {
	publisherCollections := DB.Collection("Publishers")
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	_id, _ := primitive.ObjectIDFromHex(id)
	filter := bson.M{"_id": _id}
	var publisherListing model.Publisher
	err := publisherCollections.FindOne(ctx, filter).Decode(&publisherListing)
	if err != nil {
		fmt.Println("Error:", err)
	}

	_, err = publisherCollections.DeleteOne(ctx, filter)
	if err != nil {
		panic(err)
	}
	return &publisherListing
}

func CreateUser(UserInfo model.User, password string) *model.User {
	UserCollections := DB.Collection("Users")
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	filter := bson.M{"$or": []bson.M{
		{"useranme": UserInfo.Username},
		{"email": UserInfo.Email},
	}}

	var existingUser model.User
	err := UserCollections.FindOne(ctx, filter).Decode(&existingUser)
	if err != nil {
		fmt.Println("Error:", err)
	}
	if err != mongo.ErrNoDocuments {
		fmt.Println("Error:", err)
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		fmt.Println("Error:", err)
	}

	// Insert user using a bson.M map
	inserted, err := UserCollections.InsertOne(ctx, bson.M{
		"username":         UserInfo.Username,
		"email":            UserInfo.Email,
		"hashedPassword":   string(hashedPassword),
		"fullName":         UserInfo.FullName,
		"registrationDate": time.Now().Format(time.RFC3339),
		"role":             UserInfo.Role,
	})
	if err != nil {
		fmt.Println("Error:", err)
	}

	// Convert the InsertedID to a hex string
	insertedID := inserted.InsertedID.(primitive.ObjectID).Hex()

	// Populate the return model.User struct with the hex ID and other details
	returnUser := model.User{
		ID:               insertedID,
		Username:         UserInfo.Username,
		Email:            UserInfo.Email,
		HashedPassword:   string(hashedPassword),
		FullName:         UserInfo.FullName,
		RegistrationDate: time.Now().Format(time.RFC3339),
		Role:             UserInfo.Role,
	}

	return &returnUser
}

func UpdateUser(UserId string, UserInfo model.UserUpdateInput) *model.User {
	UserCollections := DB.Collection("Users")
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	filter := bson.M{"_id": UserId}

	var exisitingUser model.User

	err := UserCollections.FindOne(ctx, filter).Decode(&exisitingUser)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			fmt.Println("Error:", err)
		}
		fmt.Println("Error:", err)
	}

	updateUserInfo := bson.M{}

	if UserInfo.CurrentPassword != nil && UserInfo.NewPassword != nil {
		//verify if the current password matches

		if bcrypt.CompareHashAndPassword([]byte(exisitingUser.HashedPassword), []byte(*UserInfo.CurrentPassword)) == nil {
			hashedPassword, err := bcrypt.GenerateFromPassword([]byte(*UserInfo.NewPassword), bcrypt.DefaultCost)
			if err != nil {
				fmt.Println("Error:", err)
			}
			updateUserInfo["hashedPassword"] = string(hashedPassword)
		} else {
			fmt.Println("Error:", err)
		}
	}

	if UserInfo.Username != nil {
		updateUserInfo["username"] = UserInfo.Username
	}
	if UserInfo.Email != nil {
		// check if the email is already taken by another user
		updateUserInfo["email"] = *UserInfo.Email
	}
	if UserInfo.FullName != nil {
		updateUserInfo["fullName"] = *UserInfo.FullName
	}
	if UserInfo.Role != nil {
		updateUserInfo["role"] = *UserInfo.Role
	}

	update := bson.M{"$set": updateUserInfo}

	results := UserCollections.FindOneAndUpdate(ctx, filter, update, options.FindOneAndUpdate().SetReturnDocument(1))

	var updatedUser model.User
	err = results.Decode(&updatedUser)
	if err != nil {
		fmt.Println("Error:", err)
	}
	return &updatedUser
}

func DeleteUser(id string) *model.User {
	UserCollections := DB.Collection("Users")
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	_id, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		fmt.Println("Error:", err)
	}
	filter := bson.M{"_id": _id}

	var user model.User
	err = UserCollections.FindOne(ctx, filter).Decode(&user)
	if err != nil {
		if err != mongo.ErrNoDocuments {
			fmt.Println("Error:", err)
		}
		fmt.Println("Error:", err)
	}

	results, err := UserCollections.DeleteOne(ctx, filter)
	if err != nil {
		fmt.Println("Error:", err)
	}

	if results.DeletedCount == 0 {
		return nil
	}

	return &user
}

func AddReview(ReviewInfo model.ReviewInput) *model.Review {
	ReviewCollections := DB.Collection("Reviews")
	BookCollections := DB.Collection("Books")
	UserCollections := DB.Collection("Users")
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	bookObjectID, _ := primitive.ObjectIDFromHex(ReviewInfo.BookID)
	bookFilter := bson.M{"_id": bookObjectID}

	var book model.Book
	err := BookCollections.FindOne(ctx, bookFilter).Decode(&book)
	if err != nil {
		fmt.Println("Error:", err)
	}

	userObjectID, _ := primitive.ObjectIDFromHex(ReviewInfo.UserID)
	userFilter := bson.M{"_id": userObjectID}

	var user model.User
	err = UserCollections.FindOne(ctx, userFilter).Decode(&user)
	if err != nil {
		log.Panic(err)
	}

	// Insert review using a bson.M map
	inserted, err := ReviewCollections.InsertOne(ctx, bson.M{
		"book":       book,
		"user":       user,
		"rating":     ReviewInfo.Rating,
		"reviewText": ReviewInfo.ReviewText,
		"reviewDate": time.Now().Format(time.RFC3339),
	})
	if err != nil {
		log.Fatal(err)
	}

	// Convert the InsertedID to a hex string
	insertedID := inserted.InsertedID.(primitive.ObjectID).Hex()

	// Populate the return model.Review struct with the hex ID and other details
	returnReview := model.Review{
		ID:         insertedID,
		Book:       &book,
		User:       &user,
		Rating:     ReviewInfo.Rating,
		ReviewText: ReviewInfo.ReviewText,
		ReviewDate: time.Now().Format(time.RFC3339),
	}

	// Update the book with total reviews and rating
	updateQuery := bson.M{
		"$inc": bson.M{
			"totalReviews": 1,
			"totalRating":  returnReview.Rating,
		},
	}
	_, err = BookCollections.UpdateOne(ctx, bson.M{"_id": book.ID}, updateQuery)
	if err != nil {
		fmt.Println("Error:", err)
	}

	return &returnReview
}

func UpdateReview(ReviewId string, ReviewInfo model.ReviewUpdateInput) *model.Review {
	ReviewCollections := DB.Collection("Reviews")
	bookCollections := DB.Collection("Books")
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	reviewId, _ := primitive.ObjectIDFromHex(ReviewId)
	filter := bson.M{"_id": reviewId}

	var existingReview model.Review
	if ReviewInfo.Rating != nil {
		err := ReviewCollections.FindOne(ctx, filter).Decode(&existingReview)
		if err != nil {
			fmt.Println("Error:", err)
		}
	}

	bookFilter := bson.M{"_id": existingReview.ID}

	updateReviewInfo := bson.M{}

	if ReviewInfo.Rating != nil {
		updateReviewInfo["rating"] = *ReviewInfo.Rating

		if *ReviewInfo.Rating != existingReview.Rating {
			ratingDiffeence := *ReviewInfo.Rating - existingReview.Rating
			bookUpdate := bson.M{
				"$inc": bson.M{
					"totalRating": ratingDiffeence,
				},
			}

			_, err := bookCollections.UpdateOne(ctx, bookFilter, bookUpdate)
			if err != nil {
				fmt.Println("Error:", err)
			}
		}
	}

	if ReviewInfo.ReviewText != nil {
		updateReviewInfo["reviewText"] = *ReviewInfo.ReviewText
	}

	if len(updateReviewInfo) == 0 {
		return &existingReview
	}
	update := bson.M{"$set": updateReviewInfo}

	results := ReviewCollections.FindOneAndUpdate(ctx, filter, update, options.FindOneAndUpdate().SetReturnDocument(1))

	var updateReview model.Review
	err := results.Decode(&updateReview)
	if err != nil {
		fmt.Println("Error:", err)
	}
	return &updateReview
}

func DeleteReview(DeleteId string) *model.Review {
	ReviewCollections := DB.Collection("Reviews")
	bookCollections := DB.Collection("Books")
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	deleteId, _ := primitive.ObjectIDFromHex(DeleteId)
	filter := bson.M{"_id": deleteId}

	var existingReview model.Review

	err := ReviewCollections.FindOne(ctx, filter).Decode(&existingReview)
	if err != nil {
		fmt.Println("Error:", err)
	}

	if existingReview.Book != nil {
		bookObjectID, err := primitive.ObjectIDFromHex(existingReview.Book.ID)
		if err != nil {
			fmt.Println("Error:", err)
		}

		bookFilter := bson.M{"_id": bookObjectID}

		bookUpdate := bson.M{
			"$inc": bson.M{
				"totalReviews": -1,
				"totalRating":  -existingReview.Rating,
			},
		}

		_, err = bookCollections.UpdateOne(ctx, bookFilter, bookUpdate)
		if err != nil {
			fmt.Println("Error:", err)
		}

	}

	result, err := ReviewCollections.DeleteOne(ctx, filter)
	if err != nil {
		fmt.Println("Error:", err)
	}

	if result.DeletedCount == 0 {
		return nil // add error here
	}

	return &existingReview
}

// func BookBorrow(BookID string, UserID string) *model.BookBorrow {
// 	BorrowCollections := DB.Collection("Borrow")
// 	BookCollections := DB.Collection("Books")
// 	UserCollections := DB.Collection("Users")
// 	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
// 	defer cancel()

// 	// check if the book exists and is available
// 	var book model.Book
// 	bookFilter := bson.M{"_id": BookID}
// 	err := BookCollections.FindOne(ctx, bookFilter).Decode(&book)
// 	if err != nil {
// 		if err == mongo.ErrNoDocuments {
// 			fmt.Println("Book not found")
// 			return nil // or return an error here
// 		}
// 		log.Fatal(err) // log and terminate on unexpected error
// 	}

// 	// Check book availability
// 	if book.Status == nil || *book.Status != model.BookStatusAvailable {
// 		fmt.Println("Book is not available")
// 		return nil
// 	}

// 	// check if user exists
// 	var user model.User
// 	userFilter := bson.M{"_id": UserID}
// 	err = UserCollections.FindOne(ctx, userFilter).Decode(&user)
// 	if err != nil {
// 		if err == mongo.ErrNoDocuments {
// 			fmt.Println("User not found")
// 			return nil // or return an error here
// 		}
// 		log.Fatal(err) // log and terminate on unexpected error
// 	}

// 	// Insert book borrow record using a bson.M map
// 	inserted, err := BorrowCollections.InsertOne(ctx, bson.M{
// 		"book":       book,
// 		"user":       user,
// 		"borrowDate": time.Now().Format(time.RFC3339),
// 		"dueDate":    time.Now().AddDate(0, 0, 14).Format(time.RFC3339),
// 		"returnDate": nil,
// 		"status":     model.BorrowStatusBorrowed,
// 	})
// 	if err != nil {
// 		log.Fatal(err)
// 	}

// 	// Convert the InsertedID to a hex string
// 	insertedID := inserted.InsertedID.(primitive.ObjectID).Hex()

// 	// Populate the return model.BookBorrow struct with the hex ID and other details
// 	returnBookBorrow := model.BookBorrow{
// 		ID:         insertedID,
// 		Book:       &book,
// 		User:       &user,
// 		BorrowDate: time.Now().Format(time.RFC3339),
// 		DueDate:    time.Now().AddDate(0, 0, 14).Format(time.RFC3339),
// 		ReturnDate: nil,
// 		Status:     model.BorrowStatusBorrowed,
// 	}

// 	session, err := db.client.StartSession()
// 	if err != nil {
// 		fmt.Println("Error:", err)
// 	}
// 	defer session.EndSession(ctx)

// 	bookStatusUpdate := bson.M{"$set": bson.M{"status": model.BookStatusCheckedOut}}
// 	userBorrowUpdate := bson.M{"$push": bson.M{"borrowedBooks": returnBookBorrow.ID}}

// 	callback := func(sessionContext mongo.SessionContext) (interface{}, error) {

// 		//update bookstatus
// 		_, err = BookCollections.UpdateOne(sessionContext, bookFilter, bookStatusUpdate)
// 		if err != nil {
// 			return nil, err
// 		}

// 		//add to user borrow
// 		_, err = UserCollections.UpdateOne(sessionContext, userFilter, userBorrowUpdate)
// 		if err != nil {
// 			return nil, err
// 		}

// 		// if both operations are successful return nil
// 		return nil, nil
// 	}

// 	err = mongo.WithSession(ctx, session, func(sessionContext mongo.SessionContext) error {
// 		_, transactionErr := session.WithTransaction(sessionContext, callback)
// 		return transactionErr
// 	})
// 	if err != nil {
// 		fmt.Println("Error:", err)
// 	}

// 	return &returnBookBorrow
// }

func BookBorrow(BookID string, UserID string) *model.BookBorrow {
	BorrowCollections := DB.Collection("Borrow")
	BookCollections := DB.Collection("Books")
	UserCollections := DB.Collection("Users")
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Check if the book exists and is available
	var book model.Book
	BookId, _ := primitive.ObjectIDFromHex(BookID)
	bookFilter := bson.M{"_id": BookId}
	err := BookCollections.FindOne(ctx, bookFilter).Decode(&book)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			fmt.Println("Book not found", BookID)
			return nil // or return an error here
		}
		fmt.Println("Error:", err) // log and terminate on unexpected error
	}

	// Check book availability
	if book.Status == nil || *book.Status != model.BookStatusAvailable {
		fmt.Println("Book is not available")
		return nil
	}

	// Check if the user exists
	var user model.User
	UserId, _ := primitive.ObjectIDFromHex(UserID)
	userFilter := bson.M{"_id": UserId}
	err = UserCollections.FindOne(ctx, userFilter).Decode(&user)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			fmt.Println("User not found")
			return nil // or return an error here
		}
		fmt.Println("Error:", err) // log and terminate on unexpected error
	}

	// Prepare the borrow record
	returnBookBorrow := model.BookBorrow{
		Book:       &book,
		User:       &user,
		BorrowDate: time.Now().Format(time.RFC3339),
		DueDate:    time.Now().AddDate(0, 0, 14).Format(time.RFC3339),
		ReturnDate: nil,
		Status:     model.BorrowStatusBorrowed,
	}

	// Start a session for the transaction
	client, err := Connect()
	if err != nil {
		fmt.Println("Error connecting to MongoDB:", err)
		return nil
	}
	session, err := client.StartSession()
	if err != nil {
		fmt.Println("Error starting session:", err)
		return nil
	}
	defer session.EndSession(ctx)

	err = mongo.WithSession(ctx, session, func(sessionContext mongo.SessionContext) error {
		// Start the transaction
		if err := session.StartTransaction(); err != nil {
			return err
		}

		// Insert the borrow record
		inserted, err := BorrowCollections.InsertOne(sessionContext, returnBookBorrow)
		if err != nil {
			return err
		}

		// Convert the InsertedID to a hex string for the BookBorrow struct
		returnBookBorrow.ID = inserted.InsertedID.(primitive.ObjectID).Hex()

		// Update book status to checked out
		bookStatusUpdate := bson.M{"$set": bson.M{"status": model.BookStatusCheckedOut}}
		_, err = BookCollections.UpdateOne(sessionContext, bookFilter, bookStatusUpdate)
		if err != nil {
			return err
		}

		// Add the borrow record ID to the user's borrowedBooks array
		userBorrowUpdate := bson.M{"$push": bson.M{"borrowedBooks": returnBookBorrow.ID}}
		_, err = UserCollections.UpdateOne(sessionContext, userFilter, userBorrowUpdate)
		if err != nil {
			return err
		}

		// Commit the transaction
		if err := session.CommitTransaction(sessionContext); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		fmt.Println("Transaction error:", err)
		return nil
	}

	return &returnBookBorrow
}

func UpdateBookBorrow(BorrowID string, BookBorrowInput model.BookBorrowUpdateInput) *model.BookBorrow {
	BorrowCollections := DB.Collection("Borrow")
	BookCollections := DB.Collection("Books")
	UserCollections := DB.Collection("Users")
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	var bookBorrow model.BookBorrow
	borrowFilter := bson.M{"_id": BorrowID}

	client, err := Connect()
	if err != nil {
		fmt.Println("Error connecting to MongoDB:", err)
		return nil
	}
	session, err := client.StartSession()
	if err != nil {
		return nil
	}
	defer session.EndSession(ctx)

	callback := func(sessionContext mongo.SessionContext) (interface{}, error) {
		borrowStatusUpdate := bson.M{"$set": bson.M{
			"status":     BookBorrowInput.Status,
			"returnDate": BookBorrowInput.ReturnDate,
		}}

		if *BookBorrowInput.Status == model.BorrowStatusReturned {

			// update borrow record
			_, err := BorrowCollections.UpdateOne(sessionContext, borrowFilter, borrowStatusUpdate)
			if err != nil {
				fmt.Println("Error:", err)
			}

			bookFilter := bson.M{"_id": bookBorrow.Book.ID}
			bookStatusUpdate := bson.M{"$set": bson.M{
				"status": model.BookStatusAvailable,
			}}

			// update book status

			_, err = BookCollections.UpdateOne(sessionContext, bookFilter, bookStatusUpdate)
			if err != nil {
				fmt.Println("Error:", err)
			}

			// remove borrowed book from userborrow record
			userFilter := bson.M{"_id": bookBorrow.User.ID}
			userBorrowUpdate := bson.M{"$pull": bson.M{
				"borrowedBooks": BorrowID,
			}}
			_, err = UserCollections.UpdateOne(sessionContext, userFilter, userBorrowUpdate)
			if err != nil {
				fmt.Println("Error:", err)
			}
		}
		return nil, nil
	}
	err = mongo.WithSession(ctx, session, func(sessionContext mongo.SessionContext) error {
		_, transactionErr := session.WithTransaction(sessionContext, callback)
		return transactionErr
	})
	if err != nil {
		fmt.Println("Error:", err)
	}

	_ = BorrowCollections.FindOne(ctx, borrowFilter).Decode(&bookBorrow)

	return &bookBorrow
}

func ReturnBook(BorrowId string) *model.BookBorrow {
	BorrowCollections := DB.Collection("Borrow")
	BookCollections := DB.Collection("Books")
	UserCollections := DB.Collection("Users")
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	var returnBook model.BookBorrow
	borrowFilter := bson.M{"_id": BorrowId}
	err := BorrowCollections.FindOne(ctx, borrowFilter).Decode(&returnBook)
	if err != nil {
		return nil
	}
	if returnBook.Status == model.BorrowStatusReturned {
		return nil
	}

	client, err := Connect()
	if err != nil {
		fmt.Println("Error connecting to MongoDB:", err)
		return nil
	}
	session, err := client.StartSession()
	if err != nil {
		return nil
	}
	defer session.EndSession(ctx)

	callback := func(sessionContext mongo.SessionContext) (interface{}, error) {

		borrowStatusUpdate := bson.M{"$set": bson.M{
			"status":     model.BorrowStatusReturned,
			"returnDate": time.Now().Format(time.RFC3339),
		}}
		_, err := BorrowCollections.UpdateOne(sessionContext, borrowFilter, borrowStatusUpdate)
		if err != nil {
			fmt.Println("Error:", err)
		}

		// update book status
		bookFilter := bson.M{"_id": returnBook.Book.ID}
		bookStatusUpdate := bson.M{"$set": bson.M{
			"status": model.BookStatusAvailable,
		}}
		_, err = BookCollections.UpdateOne(sessionContext, bookFilter, bookStatusUpdate)
		if err != nil {
			fmt.Println("Error:", err)
		}

		// remove borrowed book from userborrow record
		userFilter := bson.M{"_id": returnBook.User.ID}
		userBorrowUpdate := bson.M{"$pull": bson.M{
			"borrowedBooks": BorrowId,
		}}
		_, err = UserCollections.UpdateOne(sessionContext, userFilter, userBorrowUpdate)
		if err != nil {
			fmt.Println("Error:", err)
		}
		return nil, nil
	}

	err = mongo.WithSession(ctx, session, func(sessionContext mongo.SessionContext) error {
		_, transactionErr := session.WithTransaction(sessionContext, callback)
		return transactionErr
	})
	if err != nil {
		fmt.Println("Error:", err)
	}

	return &returnBook
}

func SearchBooks(Search string) []*model.Book {
	BookCollections := DB.Collection("Books")
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if Search == "" {
		return nil
	}

	filter := bson.M{
		"$or": []bson.M{
			{"title": bson.M{"$regex": Search, "$options": "i"}},
			{"description": bson.M{"$regex": Search, "$options": "i"}},
			{"authors.name": bson.M{"$regex": Search, "$options": "i"}},
			{"publisher.name": bson.M{"$regex": Search, "$options": "i"}},
			{"category": bson.M{"$regex": Search, "$options": "i"}},
			{"tags": bson.M{"$regex": Search, "$options": "i"}},
			{"language": bson.M{"$regex": Search, "$options": "i"}},
			{"publishedYear": Search},
		},
	}

	cur, err := BookCollections.Find(ctx, filter)
	if err != nil {
		fmt.Println("Error:", err)
	}
	defer cur.Close(ctx)

	var books []*model.Book
	for cur.Next(ctx) {
		var book model.Book
		if err := cur.Decode(&book); err != nil {
			fmt.Println("Error:", err)
		}
		books = append(books, &book)
	}

	return books
}

func CurrentUser(userID string) *model.User {
	UserCollections := DB.Collection("Users")
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	filter := bson.M{"_id": userID}

	var user model.User
	err := UserCollections.FindOne(ctx, filter).Decode(&user)
	if err != nil {
		fmt.Println("Error:", err)
	}
	return &user
}

func UserBorrows(userID string) []*model.BookBorrow {
	BorrowCollections := DB.Collection("Borrow")
	UserCollections := DB.Collection("Users")
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	userFilter := bson.M{"_id": userID}

	var currentUser model.User

	err := UserCollections.FindOne(ctx, userFilter).Decode(&currentUser)
	if err != nil {
		fmt.Println("Error:", err)
	}

	if currentUser.ID != userID &&
		currentUser.Role != model.UserRoleAdmin &&
		currentUser.Role != model.UserRoleLibrarian {
		return nil
	}

	borrowFilter := bson.M{"user._id": userID}

	cur, err := BorrowCollections.Find(ctx, borrowFilter)
	if err != nil {
		fmt.Println("Error:", err)
	}
	defer cur.Close(ctx)

	var borrows []*model.BookBorrow
	for cur.Next(ctx) {
		var borrow model.BookBorrow
		if err := cur.Decode(&borrow); err != nil {
			fmt.Println("Error:", err)
		}
		borrows = append(borrows, &borrow)
	}
	return borrows
}

func OverdueBooks(UserID string) []*model.BookBorrow {
	UserCollections := DB.Collection("Users")
	BorrowCollections := DB.Collection("Borrow")
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	userFilter := bson.M{"_id": UserID}

	var currentUser model.User
	err := UserCollections.FindOne(ctx, userFilter).Decode(&currentUser)
	if err != nil {
		fmt.Println("Error:", err)
	}

	if currentUser.ID != UserID &&
		currentUser.Role != model.UserRoleAdmin &&
		currentUser.Role != model.UserRoleLibrarian {
		return nil
	}

	overdueFilter := bson.M{
		"dueDate": time.Now().Format(time.RFC3339),
		"status":  model.BorrowStatusBorrowed,
	}

	cur, err := BorrowCollections.Find(ctx, overdueFilter)
	if err != nil {
		fmt.Println("Error:", err)
	}
	defer cur.Close(ctx)

	var overdueBooks []*model.BookBorrow
	for cur.Next(ctx) {
		var overdueBook *model.BookBorrow
		if err := cur.Decode(&overdueBook); err != nil {
			log.Fatal(nil)
		}
		overdueBooks = append(overdueBooks, overdueBook)
	}
	return overdueBooks
}
