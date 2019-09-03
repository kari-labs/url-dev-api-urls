//go:generate protoc -I .\url-dev-protobufs\ .\url-dev-protobufs\urlapi.proto --go_out=plugins=grpc:urlapi

package main

import (
	"context"
	"fmt"
	"log"
	"net"

	"github.com/google/uuid"
	"github.com/kari-labs/config"
	"github.com/kari-labs/url-dev-api-urls/urlapi"
	gonanoid "github.com/matoous/go-nanoid"
	"github.com/samsarahq/thunder/livesql"
	"github.com/samsarahq/thunder/sqlgen"
	"google.golang.org/grpc"
)

type url struct {
	UUID        uuid.UUID `sql:"uuid,primary" graphql:"id,key"`
	OriginalURL string    `sql:"originalURL"`
	ShortURL    string    `sql:"shortURL"`
	Visits      int       `sql:"visits"`
}

type server struct {
	db *livesql.LiveDB
}

func (s *server) Shorten(ctx context.Context, shreq *urlapi.ShortenRequest) (*urlapi.ShortenResponse, error) {
	if shreq.Url == "" {
		return nil, fmt.Errorf("Cannot shorten empty URL")
	}

	id, _ := gonanoid.Nanoid(9)
	url := url{
		UUID:        uuid.New(),
		OriginalURL: shreq.Url,
		ShortURL:    fmt.Sprintf("url.dev/%s", id),
		Visits:      0,
	}
	if _, err := s.db.InsertRow(ctx, &url); err != nil {
		return nil, err
	}
	return &urlapi.ShortenResponse{Shorturl: url.ShortURL}, nil
}

func (s *server) Lengthen(ctx context.Context, lereq *urlapi.LengthenRequest) (*urlapi.LengthenResponse, error) {
	if lereq.Url == "" {
		return nil, fmt.Errorf("Cannot lengthen empty URL")
	}

	var url *url

	if err := s.db.QueryRow(ctx, &url, sqlgen.Filter{"shortURL": lereq.Url}, nil); err != nil {
		return nil, err
	}

	return &urlapi.LengthenResponse{Longurl: url.OriginalURL}, nil
}

func main() {
	conf := config.New()
	sqlgenSchema := sqlgen.NewSchema()
	sqlgenSchema.MustRegisterType("public_urls", sqlgen.UniqueId, url{})

	db, err := livesql.Open(conf.DB.Host, conf.DB.Port, conf.DB.User, conf.DB.Password, conf.DB.Database, sqlgenSchema)
	if err != nil {
		panic(err)
	}

	lis, err := net.Listen("tcp", "0.0.0.0:3009")
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}

	apiservice := server{
		db: db,
	}

	grpcserver := grpc.NewServer()
	urlapi.RegisterShortenAPIServiceServer(grpcserver, &apiservice)
	if err := grpcserver.Serve(lis); err != nil {
		log.Fatalf("failed to serve: %v", err)
	}
}
