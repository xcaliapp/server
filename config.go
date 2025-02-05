package main

type drawingStoreType int

const (
	LOCAL_GIT drawingStoreType = iota
	GITLAB
	S3
)

type passwordCredentials struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type options struct {
	port            int
	passwordCreds   []passwordCredentials
	drawingStoreTyp drawingStoreType
	drawingStoreKey string
}
