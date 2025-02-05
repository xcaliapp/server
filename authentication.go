package main

import (
	"encoding/base64"
	"strings"

	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog"
)

const userKey = "iconrepo-user"

type User struct {
	Username string
}

func decodeBasicAuthnHeaderValue(headerValue string) (userid string, password string, decodeOK bool) {
	s := strings.SplitN(headerValue, " ", 2)
	if len(s) != 2 {
		return "", "", false
	}

	b, err := base64.StdEncoding.DecodeString(s[1])
	if err != nil {
		return "", "", false
	}

	pair := strings.SplitN(string(b), ":", 2)
	if len(pair) != 2 {
		return "", "", false
	}

	return pair[0], pair[1], true
}

type basicConfig struct {
	passwordCreds []passwordCredentials
}

func checkBasicAuthentication(options basicConfig) func(c *gin.Context) {
	return func(c *gin.Context) {
		logger := zerolog.Ctx(c.Request.Context())
		authenticated := false

		session := sessions.Default(c)
		user := session.Get(userKey)
		logger.Debug().Bool("isAuthenticated", authenticated).Send()
		if user != nil {
			authenticated = true
		} else {
			authnHeaderValue, hasHeader := c.Request.Header["Authorization"]
			logger.Debug().Bool("hasHeader", hasHeader).Send()
			if hasHeader {
				username, password, decodeOK := decodeBasicAuthnHeaderValue(authnHeaderValue[0])
				logger.Debug().Bool("headerCouldBeDecoded", decodeOK).Send()
				if decodeOK {
					logger.Debug().Str("username", username).Send()
					logger.Debug().Int("passwordCredentialsList length", len(options.passwordCreds)).Send()
					for _, pc := range options.passwordCreds {
						logger.Debug().Str("currentUserName", pc.Username).Send()
						if pc.Username == username && pc.Password == password {
							session.Set(userKey, User{username})
							session.Save()
							authenticated = true
							break
						}
					}
				}
			}
		}
		session.Save()

		if authenticated {
			c.Next()
		} else {
			c.Header("WWW-Authenticate", "Basic")
			c.AbortWithStatus(401)
		}
	}
}
