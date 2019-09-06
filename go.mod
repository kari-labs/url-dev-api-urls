module github.com/kari-labs/url-dev-api-urls

go 1.12

require (
	github.com/go-sql-driver/mysql v1.4.1
	github.com/go-xorm/xorm v0.7.6
	github.com/golang/protobuf v1.3.2
	github.com/google/uuid v1.1.1
	github.com/joho/godotenv v1.3.0
	github.com/kari-labs/config v0.0.0-20190902212622-6d4ada69cb78
	github.com/matoous/go-nanoid v1.1.0
	google.golang.org/grpc v1.23.0
	gopkg.in/check.v1 v1.0.0-20190902080502-41f04d3bba15 // indirect
	xorm.io/core v0.7.0
)

replace github.com/kari-labs/config => ../config
