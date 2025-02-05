package main

type passwordCredentials struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type options struct {
	port          int
	passwordCreds []passwordCredentials
}
