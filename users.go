package main

import (
	"log"
	"net/http"
	"time"

	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"

	"github.com/jinzhu/gorm"

	"golang.org/x/crypto/bcrypt"
)

type user struct {
	gorm.Model
	Email    string
	Name     string
	Password string
	Open     bool
	Level    int // 999 is admin
}

func genUserStartGetHandler(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		u := &user{}
		db.First(u)
		log.Println(u, u == nil)
		if !(*u == user{}) {
			abortWith404(c)
			return
		}

		c.File("./views/start.html")
	}
}

func genUserStartPostHandler(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		u := &user{}
		db.First(u)
		log.Println(u, u == nil)
		if !(*u == user{}) {
			abortWith404(c)
			return
		}

		u.Name = c.PostForm("name")
		pwd := c.PostForm("password")
		hashed, _ := bcrypt.GenerateFromPassword([]byte(pwd), bcrypt.DefaultCost)
		u.Password = string(hashed)
		u.Email = c.PostForm("email")
		u.Open = true
		u.Level = 999

		db.Create(u)
		c.HTML(http.StatusOK, "location.html", gin.H{
			"NextURL": "/login",
		})
	}
}

func genUserLoginGetHandler(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.File("views/login.html")
	}
}

func genUserLoginPostHandler(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		email := c.PostForm("email")
		password := c.PostForm("password")

		u := &user{}
		db.Model(u).Where("email = ?", email).First(u)
		if (*u == user{}) {
			c.Status(http.StatusUnauthorized)
			return
		}

		if err := bcrypt.CompareHashAndPassword([]byte(u.Password), []byte(password)); err != nil {
			c.Status(http.StatusUnauthorized)
			return
		}

		sess := sessions.Default(c)
		sess.Set("info", sessInfo{
			User:      *u,
			Timestamp: time.Now(),
		})
		err := sess.Save()
		if err != nil {
			log.Println("ERROR save session", err)
		}
		c.HTML(http.StatusOK, "location.html", gin.H{
			"NextURL": "/dashboard",
		})
	}
}

func genUserChangePasswordHandler(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {

	}
}

func genUserLogoutHandler(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {

	}
}

type sessInfo struct {
	User      user      `json:"user"`
	Timestamp time.Time `json:"ts"`
}
