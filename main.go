package main

import (
	"context"
	"fmt"
	"github/PopsanAemaos/goTodo/auth"
	"github/PopsanAemaos/goTodo/todo"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
	"golang.org/x/time/rate"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

var (
	buildcommit = "dev"
	buildtime   = time.Now().String()
)

// go build\ -ldflags"-X main.buildcommit='git rev-parse --short HEAD` \ -X main.buildtime=`date"+%Y-%m-%dT&H:&M:S%2:00"`"\ -o app

func main() {
	_, err := os.Create("/tmp/live")
	if err != nil {
		log.Fatal(err)
	}
	defer os.Remove("/tmp/live")

	err = godotenv.Load("local.env")
	if err != nil {
		fmt.Printf("please consider environment variables:%s", err)
	}
	db, err := gorm.Open(mysql.Open(os.Getenv("MARIADB")), &gorm.Config{})
	if err != nil {
		fmt.Println(err)
		panic("failed to conect database")
	}

	db.AutoMigrate(&todo.Todo{})

	// db.Create(&User{Name: "Popsan"})
	r := gin.Default()
	r.GET("/healthz", func(c *gin.Context) {
		c.Status(200)
	})

	r.GET("/tokenz", auth.AccessToken(os.Getenv("SIGN")))
	handler := todo.NewTodoHandler(db)
	protected := r.Group("", auth.Protect([]byte(os.Getenv("SIGN"))))
	protected.POST("/todos", handler.NewTask)
	r.GET("/limitz", limitedHandler)

	r.GET("/x", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"buildcommit": buildcommit,
			"buildtime":   buildtime,
		})
	})
	// ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	// defer stop()
	s := &http.Server{
		Addr:           ":" + os.Getenv("PORT"),
		Handler:        r,
		ReadTimeout:    10 * time.Second,
		WriteTimeout:   10 * time.Second,
		MaxHeaderBytes: 1 << 20,
	}
	go func() {
		if err := s.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("listen: &sin", err)
		}
	}()
	// <-ctx.Done()
	// stop()
	gracefulStop := make(chan os.Signal, 1)
	signal.Notify(gracefulStop, syscall.SIGTERM)
	signal.Notify(gracefulStop, syscall.SIGINT)

	<-gracefulStop

	fmt.Println("shutting downIgracefully, press Ctrl+C again to force")
	timeoutContext, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := s.Shutdown(timeoutContext); err != nil {
		fmt.Println(err)
	}
}

var limiter = rate.NewLimiter(5, 5)

func limitedHandler(c *gin.Context) {
	if !limiter.Allow() {
		c.AbortWithStatus(http.StatusTooManyRequests)
		return
	}
	c.JSON(200, gin.H{
		"message": "pong",
	})
}

// echo "GET http://localhost:2020/limitz" | vegeta attack -rate=10/s -duration=1s | vegeta report
