package main

import (
	"context"
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/sessions"
	"github.com/gorilla/websocket"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

var (
	client              *mongo.Client
	chatroomsCollection *mongo.Collection
	usersCollection     *mongo.Collection
	store               = sessions.NewCookieStore([]byte("secret-key"))
	upgrader            = websocket.Upgrader{}
)

type Chatroom struct {
	RoomID   string   `bson:"room_id"`
	RoomName string   `bson:"room_name"`
	Users    []string `bson:"users"`
	Messages []string `bson:"messages"`
}

type User struct {
	Username string   `bson:"username"`
	Password string   `bson:"password"`
	Rooms    []string `bson:"rooms"`
}

func init() {
	clientOptions := options.Client().ApplyURI("mongodb://localhost:27017")
	client, err := mongo.Connect(context.Background(), clientOptions)
	if err != nil {
		log.Fatal(err)
	}
	err = client.Ping(context.Background(), nil)
	if err != nil {
		log.Fatal(err)
	}
	chatroomsCollection = client.Database("chatdb").Collection("chatrooms")
	usersCollection = client.Database("chatdb").Collection("users")
}

func ChatroomPage(ctx *gin.Context) {

	cursor, err := chatroomsCollection.Find(context.Background(), bson.D{})
	if err != nil {
		ctx.JSON(500, gin.H{"error": err.Error()})
		return
	}
	var chatrooms []Chatroom
	if err := cursor.All(context.Background(), &chatrooms); err != nil {
		ctx.JSON(500, gin.H{"error": err.Error()})
		return
	}

	session, _ := store.Get(ctx.Request, "chat-session")
	username, ok := session.Values["username"].(string)

	ctx.HTML(200, "index.html", gin.H{
		"Chatrooms": chatrooms,
		"Username":  username,
		"LoggedIn":  ok,
	})
}

func DisplayRooms(ctx *gin.Context) {
	roomid := ctx.Param("roomid")
	session, _ := store.Get(ctx.Request, "chat-session")
	username, _ := session.Values["username"].(string)

	var room Chatroom
	chatroomsCollection.FindOne(ctx, bson.M{"room_id": roomid}).Decode(&room)

	ctx.HTML(200, "room.html", gin.H{"RoomName": room.RoomName, "RoomID": room.RoomID, "Username": username,
		"Messages": room.Messages,
	})
}

func WebsocketHandler(ctx *gin.Context) {
	roomid := ctx.Param("roomid")
	session, _ := store.Get(ctx.Request, "chat-session")
	username, ok := session.Values["username"].(string)

	if !ok {
		ctx.JSON(http.StatusUnauthorized, gin.H{"error": "User not logged in"})
		return
	}

	conn, err := upgrader.Upgrade(ctx.Writer, ctx.Request, nil)
	if err != nil {
		log.Println("WebSocket upgrade error:", err)
		return
	}
	defer conn.Close()

	log.Printf("User %s connected to room %s", username, roomid)

	for {
		_, message, err := conn.ReadMessage()
		if err != nil {
			log.Println("WebSocket read error:", err)
			break
		}
		var chatroom Chatroom
		chatroomsCollection.FindOne(ctx, bson.M{"room_id": roomid}).Decode(&chatroom)
		chatroom.Messages = append(chatroom.Messages, string(message))
		_, err = chatroomsCollection.UpdateOne(ctx, bson.M{"room_id": roomid}, bson.M{
			"$set": bson.M{"messages": chatroom.Messages},
		})
		log.Printf("[%s] %s: %s", roomid, username, message)

	}
}

func PersistRooms(ctx *gin.Context) {
	roomid := ctx.Param("roomid")
	session, _ := store.Get(ctx.Request, "chat-session")
	username, ok := session.Values["username"].(string)

	if !ok {
		ctx.Redirect(http.StatusFound, "/login")
		return
	}
	// Find the chatroom by roomid
	var room Chatroom
	err := chatroomsCollection.FindOne(ctx, bson.M{"room_id": roomid}).Decode(&room)
	if err != nil {
		ctx.JSON(http.StatusNotFound, gin.H{"error": "Room not found"})
		return
	}
	// Add the user to the room's Users array
	room.Users = append(room.Users, username)
	_, err = chatroomsCollection.UpdateOne(ctx, bson.M{"room_id": roomid}, bson.M{
		"$set": bson.M{"users": room.Users},
	})
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	// Update the user's Rooms array
	var user User
	err = usersCollection.FindOne(ctx, bson.M{"username": username}).Decode(&user)
	if err != nil {
		ctx.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}
	user.Rooms = append(user.Rooms, roomid)
	_, err = usersCollection.UpdateOne(ctx, bson.M{"username": username}, bson.M{
		"$set": bson.M{"rooms": user.Rooms},
	})
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	// Redirect to the room page
	ctx.HTML(200, "room.html", gin.H{
		"RoomName": room.RoomName,
		"Username": username,
		"Messages": room.Messages,
	})
}

func main() {
	r := gin.Default()
	r.Static("/static", "./public/static")
	r.LoadHTMLGlob("public/templates/*.html")

	r.GET("/", ChatroomPage)
	r.GET("/signup", RenderSignup)
	r.POST("/signup", SignupBackend)
	r.GET("/login", RenderLogin)
	r.POST("/login", LoginBackend)
	r.GET("/logout", Logout)
	r.POST("/:roomid", DisplayRooms)
	r.GET("/ws/:roomid", WebsocketHandler)

	r.GET("/:roomid", PersistRooms)

	r.Run(":8080")
}
