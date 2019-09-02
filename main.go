package main

import (
	"context"
	"fmt"
	"log"
	"net/http"

	"github.com/joho/godotenv"
	"github.com/kari-labs/config"

	"github.com/samsarahq/thunder/sqlgen"

	"github.com/google/uuid"
	gonanoid "github.com/matoous/go-nanoid"
	"github.com/samsarahq/thunder/graphql"
	"github.com/samsarahq/thunder/graphql/graphiql"
	"github.com/samsarahq/thunder/graphql/introspection"
	"github.com/samsarahq/thunder/graphql/schemabuilder"
	"github.com/samsarahq/thunder/livesql"
)

type Url struct {
	UUID        uuid.UUID `sql:"uuid,primary" graphql:"id,key"`
	OriginalURL string    `sql:"originalURL"`
	ShortURL    string    `sql:"shortURL"`
	Visits      int       `sql:"visits"`
}

type Server struct {
	db *livesql.LiveDB
}

func (s *Server) registerQuery(schema *schemabuilder.Schema) {
	obj := schema.Query()

	obj.FieldFunc("getAllURLs", func(ctx context.Context) ([]*Url, error) {
		var urls []*Url
		if err := s.db.Query(ctx, &urls, nil, nil); err != nil {
			return nil, err
		}
		return urls, nil
	})
}

func (s *Server) registerMutation(schema *schemabuilder.Schema) {
	obj := schema.Mutation()

	obj.FieldFunc("createShortURL", func(ctx context.Context, args struct{ OriginalURL string }) (*Url, error) {
		id, _ := gonanoid.Nanoid(9)
		url := Url{
			UUID:        uuid.New(),
			OriginalURL: args.OriginalURL,
			ShortURL:    fmt.Sprintf("url.dev/%s", id),
			Visits:      0,
		}
		if _, err := s.db.InsertRow(ctx, &url); err != nil {
			return nil, err
		}
		return &url, nil
	})
}

func (s *Server) schema() *graphql.Schema {
	builder := schemabuilder.NewSchema()
	s.registerQuery(builder)
	s.registerMutation(builder)
	return builder.MustBuild()
}

func init() {
	if err := godotenv.Load(); err != nil {
		log.Print("No .env file found, please create one with values")
	}
}

func main() {
	conf := config.New()
	sqlgenSchema := sqlgen.NewSchema()
	sqlgenSchema.MustRegisterType("public_urls", sqlgen.UniqueId, Url{})

	db, err := livesql.Open(conf.DB.Host, conf.DB.Port, conf.DB.User, conf.DB.Password, conf.DB.Database, sqlgenSchema)
	if err != nil {
		panic(err)
	}

	server := &Server{db: db}
	schema := server.schema()
	introspection.AddIntrospectionToSchema(schema)

	http.Handle("/graphql", graphql.Handler(schema))
	http.Handle("/graphiql/", http.StripPrefix("/graphiql/", graphiql.Handler()))
	fmt.Println("Listening on port 3050")
	http.ListenAndServe(":3030", nil)
}
