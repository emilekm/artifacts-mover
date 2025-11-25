package internal

//go:generate go run go.uber.org/mock/mockgen -source=./uploader.go -destination=./uploader_mock.go -package=internal Uploader

type Uploader interface {
	Upload(Round) error
}
