//go:generate protoc -I .\url-dev-protobufs\ .\url-dev-protobufs\urlapi.proto --go_out=plugins=grpc:urlapi

package main

import (
	"context"
	"fmt"
	"log"
	"net"

	_ "github.com/go-sql-driver/mysql"
	"github.com/go-xorm/xorm"
	"github.com/google/uuid"
	"github.com/joho/godotenv"
	"github.com/kari-labs/config"
	"github.com/kari-labs/url-dev-api-urls/urlapi"
	gonanoid "github.com/matoous/go-nanoid"
	"google.golang.org/grpc"
	"xorm.io/core"
)

// publicURL represents a URL created by anonymous users
type publicURL struct {
	// (BUG: The UUID's are currently being stored with quotes around them in the database for some reason.
	//	Probably has something to do with the way XORM converts them to 'varchar(255)')
	UUID        uuid.UUID `xorm:"varchar(255) not null pk 'uuid'"`
	OriginalURL string    `xorm:"'originalURL'"`
	ShortURL    string    `xorm:"'shortURL'"`
	Visits      int       `xorm:"'visits'"`
}

// TableName is a method used by Xorm to determine the correct SQL tablename to use when syncing the struct
func (*publicURL) TableName() string {
	return "public_urls"
}

// server contains a reference to our database connection as well as our gRPC methods
type server struct {
	db *xorm.Engine
}

// Shorten takes a URL and RequestType of RETRIEVE or FORCE_GEN
// Using the type of the request, the method either attempts to retrieve and existing
// 		URL or just generates a new one
func (s *server) Shorten(ctx context.Context, req *urlapi.ShortenRequest) (*urlapi.ShortenResponse, error) {
	if req.GetUrl() == "" {
		return nil, fmt.Errorf("Cannot shorten empty URL")
	}

	if req.GetType() == urlapi.ShortenRequest_RETRIEVE {
		if has, err := s.db.Where("originalURL = ?", req.GetUrl()).Exist(&publicURL{}); err != nil {
			return nil, fmt.Errorf("Error checking database for existing short URL: %v", err)
		} else if has {
			var temp publicURL
			if _, err := s.db.Where("originalURL = ?", req.GetUrl()).Get(&temp); err != nil {
				return nil, fmt.Errorf("Error retrieving existing short URL from database: %v", err)
			}

			return &urlapi.ShortenResponse{
				Shorturl: temp.ShortURL,
			}, nil
		}
	}

	// URL generation happens in 2 cases
	// 1. User attempts to create a shortened URL. The client passes in a Retrieve request by default, attempting to return
	// 		an already existing short URL. A short URL doesn't already exist for this URL so we generate one.
	// 		(TODO: Not sure if this should be default behavior. Could return message to client saying that a URL hasn't been created yet?)
	// 2. User sends a request with type FORCE_GEN. This forces the server to generate a new short URL by default.

	id, _ := gonanoid.Nanoid(9)
	temp := publicURL{
		UUID:        uuid.New(),
		OriginalURL: req.Url,
		ShortURL:    fmt.Sprintf("url.dev/%s", id),
		Visits:      0,
	}

	// (BUG: We shoudn't need to specify the table here, XORM should use the TableName() method to resolve this.
	//	Not sure why I have to manually specify.)
	if _, err := s.db.Table("public_urls").Insert(temp); err != nil {
		return nil, fmt.Errorf("Error inserting new short URL to database: %v", err)
	}
	return &urlapi.ShortenResponse{Shorturl: temp.ShortURL}, nil
}

// Lengthen is a method which takes a short URL to be transformed back into its original form
// The resolver uses this to redirect users to the correct locations
func (s *server) Lengthen(ctx context.Context, lereq *urlapi.LengthenRequest) (*urlapi.LengthenResponse, error) {
	if lereq.Url == "" {
		return nil, fmt.Errorf("Cannot lengthen empty URL")
	}

	var temp publicURL

	if _, err := s.db.Where("shortURL = ?", lereq.Url).Get(&temp); err != nil {
		fmt.Println(err)
		return nil, err
	}

	return &urlapi.LengthenResponse{Longurl: temp.OriginalURL}, nil
}

func main() {
	// Load in environment variables for development
	err := godotenv.Load()
	if err != nil {
		log.Fatal("Error loading .env file")
	}

	// Creating a new config for connection to DB
	conf := config.New()

	// Initialize connection
	engine, err := xorm.NewEngine("mysql", conf.CreateConnectionString())
	if err != nil {
		log.Fatalf("error connecting to database: %v", err)
	}

	// By default XORM uses 'SnakeMapper', which attempts to put underscores inbetween words.
	// It does this by inserting an underscore before each capital letter. This causes problems with the acronym URL
	//		and other words we use.
	// GonicMapper just doesn't insert underscores between words, instead lowercasing them.
	// See docs for more info - https://github.com/go-xorm/manual-en-US/blob/master/chapter-02/1.mapping.md
	engine.SetTableMapper(core.GonicMapper{})

	// Sync DB engine to use proper table for queries, inserts.
	// (TODO: We should be using database sessions and having different structs for public_urls, org_urls, etc
	// 		This current method isn't good because this service will ideally handle all URL interaction, not just
	//		"public URL" creation)
	err = engine.Sync2(new(publicURL))
	if err != nil {
		log.Fatalf("error syncing struct to database table: %v", err)
	}

	// Initialize server with DB connection
	apiservice := server{
		db: engine,
	}

	// Initialize a TCP listener for the gRPC server
	lis, err := net.Listen("tcp", "0.0.0.0:3009")
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}

	// Initialize, register, and start gRPC server
	grpcserver := grpc.NewServer()
	urlapi.RegisterShortenAPIServiceServer(grpcserver, &apiservice)
	if err := grpcserver.Serve(lis); err != nil {
		log.Fatalf("failed to serve: %v", err)
	}
}
