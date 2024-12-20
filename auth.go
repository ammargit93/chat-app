package main

import (
	"context"
	"net/http"

	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/bson"
)

func RenderSignup(ctx *gin.Context) {
	ctx.HTML(200, "signup.html", nil)
}

func SignupBackend(ctx *gin.Context) {
	username := ctx.PostForm("username")
	password := ctx.PostForm("password")
	var existingUser User
	err := usersCollection.FindOne(context.Background(), bson.M{"username": username}).Decode(&existingUser)
	if err == nil {
		ctx.JSON(400, gin.H{"error": "Username already exists"})
		return
	}
	newUser := User{
		Username: username,
		Password: password,
		Rooms:    nil,
	}
	_, err = usersCollection.InsertOne(context.Background(), newUser)
	if err != nil {
		ctx.JSON(500, gin.H{"error": err.Error()})
		return
	}
	ctx.Redirect(http.StatusFound, "/login")
}

func RenderLogin(ctx *gin.Context) {
	ctx.HTML(200, "login.html", nil)
}

func LoginBackend(ctx *gin.Context) {
	username := ctx.PostForm("username")
	password := ctx.PostForm("password")
	var user User
	err := usersCollection.FindOne(context.Background(), bson.M{"username": username}).Decode(&user)
	if err != nil || user.Password != password {
		ctx.JSON(400, gin.H{"error": "Invalid credentials"})
		return
	}

	session, _ := store.Get(ctx.Request, "chat-session")
	session.Values["username"] = user.Username
	session.Save(ctx.Request, ctx.Writer)

	ctx.Redirect(http.StatusFound, "/")
}

func Logout(ctx *gin.Context) {
	session, _ := store.Get(ctx.Request, "chat-session")
	delete(session.Values, "username")
	session.Save(ctx.Request, ctx.Writer)
	ctx.Redirect(http.StatusFound, "/login")
}
