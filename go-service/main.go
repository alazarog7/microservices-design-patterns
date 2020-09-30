package main

import (
	"context"
	"fmt"
	"github.com/ArthurHlt/go-eureka-client/eureka"
	"github.com/Piszmog/cloudconfigclient"
	"github.com/dgrijalva/jwt-go"
	"github.com/joho/godotenv"
	"github.com/labstack/echo-contrib/prometheus"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"github.com/labstack/gommon/log"
	"github.com/sirupsen/logrus"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"io/ioutil"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

type Post struct {
	ID                 primitive.ObjectID    `bson:"_id" json:"id" form:"id" query:"id"`
	Name               string    `json:"name" form:"name" query:"name"`
	CreatedDate        time.Time `json:"createdDate" form:"createDate" query:"createDate"`
	LastModifiedDate   time.Time `json:"lastModifiedDate"`
	CreatedByUser      string    `json:"createdByUser"`
	LastModifiedByUser string    `json:"lastModifiedByUser"`
}

type JsonResponse struct {
	Status	string	 `json:"status"`
}

var (
	loadEnvFlag = true
	e           = echo.New()
	client      = connectMongo()
	collection  = client.Database(getEnv("MONGODB_DATABASE")).Collection("posts")
)

func connectMongo() *mongo.Client {
	// Mongodb
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	client, err := mongo.Connect(ctx, options.Client().ApplyURI(getEnv("MONGODB_URI")))
	if err != nil {
		panic(err)
	}
	return client
}



//----------
// Handlers
//----------
func createDefaultPosts() {
	count, _ := collection.CountDocuments(context.TODO(), bson.D{})
	if count > 0 {
		return
	}
	var posts = []Post{
		{
			ID: primitive.NewObjectID(),
			Name:               "Golang",
			CreatedDate:        time.Now(),
			CreatedByUser:      "default@admin.com",
		},
		{
			ID: primitive.NewObjectID(),
			Name:               "Test",
			CreatedDate:        time.Now(),
			CreatedByUser:      "default@admin.com",
		},
	}

	for _, post := range posts {
		_, err := collection.InsertOne(context.TODO(), post)
		if err != nil {
			log.Fatal(err)
		}
	}
}

func getAllPosts(c echo.Context) error {
	ctx := context.TODO()
	cur, err := collection.Find(ctx, bson.D{})
	if err != nil {
		return err
	}
	var posts []*Post
	for cur.Next(ctx) {
		var post Post
		if err := cur.Decode(&post); err != nil {
			return err
		}
		posts = append(posts, &post)
	}
	return c.JSON(http.StatusOK, posts)
}

func createPost(c echo.Context) error {
	u := new(Post)
	if err := c.Bind(u); err != nil {
		return err
	}
	u.CreatedDate = time.Now()
	u.LastModifiedByUser = ""
	claims := getAuthUser(c)
	u.CreatedByUser = claims["sub"].(string)
	if _, err := collection.InsertOne(context.TODO(), u); err != nil {
		return err
	}
	return c.JSON(http.StatusCreated, u)
}

func getAuthUser(c echo.Context) jwt.MapClaims {
	user := c.Get("user").(*jwt.Token)
	claims := user.Claims.(jwt.MapClaims)
	return claims
}

func getPost(c echo.Context) error {
	id := c.Param("id")
	post := Post{}
	filter := bson.D{{"_id", id}}
	if err := collection.FindOne(context.TODO(), filter).Decode(&post); err != nil {
		return err
	}
	if post.ID.IsZero() {
		return echo.NewHTTPError(http.StatusNotFound, fmt.Sprintf("Not found ID: %v", id))
	}
	return c.JSON(http.StatusOK, post)
}

func updatePost(c echo.Context) error {
	id := c.Param("id")
	post := Post{}
	ctx := context.TODO()
	filter := bson.D{{"_id", id}}
	if err := collection.FindOne(ctx, filter).Decode(&post); err != nil {
		return err
	}
	if post.ID.IsZero() {
		return echo.NewHTTPError(http.StatusNotFound, fmt.Sprintf("Not found ID: %v", id))
	}

	u := new(Post)
	if err := c.Bind(u); err != nil {
		return err
	}

	u.LastModifiedDate = time.Now()
	u.LastModifiedByUser = getAuthUser(c)["sub"].(string)
	if _, err := collection.UpdateOne(ctx, filter, u); err != nil {
		return err
	}
	return c.JSON(http.StatusOK, u)
}

func deletePost(c echo.Context) error {
	id := c.Param("ID")
	if _, err := collection.DeleteOne(context.TODO(), id); err != nil {
		return err
	}
	return c.NoContent(http.StatusNoContent)
}

func healthCheck(c echo.Context) error  {
	json := JsonResponse{Status: "OK"}
	return c.JSON(http.StatusOK, json)
}

func getEnv(key string) string {
	if loadEnvFlag {
		loadEnv()
		loadEnvFlag = false
	}
	value := os.Getenv(key)
	if "" == value {
		valueEnv, exists := os.LookupEnv(key)
		if  !exists {
			panic("Not found variable: " + key)
		}
		value = valueEnv
	}
	fmt.Print(fmt.Sprintf("Env = %+v\tvalue = %v\n", key, value))
	return value
}

func getEnvAsInt(key string) int {
	value, err := strconv.Atoi(getEnv(key))
	if err != nil {
		panic("Not found variable: " + key)
	}

	return value
}

func loadEnv()  {
	// loads values from .env into the system
	env := ".env"
	environment := os.Getenv("ENVIRONMENT")
	if environment != "" {
		env += "." + environment
	}

	fmt.Print(fmt.Sprintf("Env = %+v\n", env))
	if err := godotenv.Load(env); err != nil {
		panic(err)
	}
}

func init()  {
	appId := processEurekaClient()

	profiles, config := processConfigClient(appId)

	middlewareObj := processJwt(profiles, config)

	processRestApi(middlewareObj)
}

func processEurekaClient() string {
	logrus.SetLevel(logrus.DebugLevel)
	logrus.SetFormatter(&logrus.TextFormatter{ForceColors: true})
	client := eureka.NewClient([]string{
		getEnv("EUREKA_SERVER"), //From a spring boot based eureka server
		// add others servers here
	})
	log.Infof("Eureka Client = %+v", client)
	appId := getEnv("APP_ID")
	instance := eureka.NewInstanceInfo(getEnv("HOSTNAME"),
		appId,
		getEnv("IP_ADDRESS"),
		getEnvAsInt("SERVER_PORT"), 30, false) //Create a new instance to register
	instance.Metadata = &eureka.MetaData{
		Map: make(map[string]string),
	}
	log.Infof("Instance = %+v", instance)
	if err := client.RegisterInstance(appId, instance); err != nil {
		panic(err)
	}
	return appId
}

func processConfigClient(appId string) (string, cloudconfigclient.Source) {
	springConfigUrl := getEnv("SPRING_CLOUD_CONFIG_URI")
	springConfigUrl = fmt.Sprintf("%v?X-Encrypt-Key=%v", springConfigUrl, getEnv("X_ENCRYPT_KEY"))
	configClient, err := cloudconfigclient.NewLocalClient(&http.Client{}, []string{springConfigUrl})
	log.Infof("ConfigClient = %+v", configClient)
	if err != nil {
		panic(err)
	}

	profiles := getEnv("SPRING_PROFILES_ACTIVE")
	config, err := configClient.GetConfiguration(appId, []string{profiles})
	log.Infof("CONFIG Client = %+v", config)
	return profiles, config
}

func processJwt(profiles string, config cloudconfigclient.Source) echo.MiddlewareFunc {
	//JWT
	middlewareObj := middleware.JWT([]byte("secret"))
	if strings.Contains(profiles, "prod") {
		bytes, err := ioutil.ReadFile(getEnv("PUBLIC_KEY_PATH"))
		if err != nil {
			panic(err)
		}
		pem, err := jwt.ParseRSAPublicKeyFromPEM(bytes)
		if err != nil {
			panic(err)
		}
		middlewareObj = middleware.JWTWithConfig(middleware.JWTConfig{
			SigningKey:    pem,
			SigningMethod: "RS256",
		})
	} else {
		secretKey := config.PropertySources[0].Source["security.oauth2.resource.jwt.keyValue"]
		if secretKey == nil {
			secretKey = config.PropertySources[1].Source["security.oauth2.resource.jwt.keyValue"]
		}
		if secretKey == nil {
			panic("Not found secretKey")
		}
		middlewareObj = middleware.JWT([]byte(secretKey.(string)))
	}
	return middlewareObj
}

// urlSkipper ignores metrics route on some middleware
func urlSkipper(c echo.Context) bool {
	if strings.HasPrefix(c.Path(), "/actuator") {
		return true
	}
	return false
}

func processRestApi(middlewareObj echo.MiddlewareFunc) {
	// Middleware
	e.Use(middleware.Recover(),
		middleware.Logger())
	//middleware.CSRF())

	// Routes
	e.Logger.SetLevel(log.DEBUG)
	e.GET("/api/posts", getAllPosts, middlewareObj)
	e.POST("/api/posts", createPost, middlewareObj)
	e.GET("/api/posts/:id", getPost, middlewareObj)
	e.PUT("/api/posts/:id", updatePost, middlewareObj)
	e.DELETE("/api/posts/:id", deletePost, middlewareObj)
	e.GET("/actuator/info", healthCheck)
	e.GET("/actuator/health", healthCheck)

	p := prometheus.NewPrometheus("echo", urlSkipper)
	p.MetricsPath = "/actuator/metrics"
	p.Use(e)
}

func main() {
	createDefaultPosts()

	// Start server
	e.Logger.Fatal(e.Start(":"+getEnv("SERVER_PORT")))
}
