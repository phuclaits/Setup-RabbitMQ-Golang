package main

import (
	"database/sql"
	"fmt"
	"log"
	"net"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	_ "github.com/lib/pq"
	"github.com/rabbitmq/amqp091-go"
)

var (
	rabbitCh *amqp091.Channel
	db       *sql.DB
)
func main() {
	// Chờ RabbitMQ sẵn sàng
	waitForRabbitMQ("rabbitmq", "5672")

	// Kết nối RabbitMQ
	rabbitConn, err := amqp091.Dial("amqp://guest:guest@rabbitmq:5672/")
	if err != nil {
		log.Fatalf("Failed to connect to RabbitMQ: %v", err)
	}
	defer rabbitConn.Close()

	rabbitCh, err = rabbitConn.Channel()
	if err != nil {
		log.Fatalf("Failed to open a channel: %v", err)
	}
	defer rabbitCh.Close()

	// Khai báo Exchange và Queue
	err = rabbitCh.ExchangeDeclare("exchange1", "direct", true, false, false, false, nil)
	if err != nil {
		log.Fatalf("Failed to declare exchange1: %v", err)
	}

	err = rabbitCh.ExchangeDeclare("exchange2", "direct", true, false, false, false, nil)
	if err != nil {
		log.Fatalf("Failed to declare exchange2: %v", err)
	}

	_, err = rabbitCh.QueueDeclare("queue1", true, false, false, false, nil)
	if err != nil {
		log.Fatalf("Failed to declare queue1: %v", err)
	}

	_, err = rabbitCh.QueueDeclare("queue2", true, false, false, false, nil)
	if err != nil {
		log.Fatalf("Failed to declare queue2: %v", err)
	}

	err = rabbitCh.QueueBind("queue1", "routingKey1", "exchange1", false, nil)
	if err != nil {
		log.Fatalf("Failed to bind queue1: %v", err)
	}

	err = rabbitCh.QueueBind("queue2", "routingKey2", "exchange2", false, nil)
	if err != nil {
		log.Fatalf("Failed to bind queue2: %v", err)
	}

	// Kết nối PostgreSQL
	db, err = sql.Open("postgres", "postgres://root:root@postgres:5432/testRabbit?sslmode=disable")
	if err != nil {
		log.Fatalf("Failed to connect to PostgreSQL: %v", err)
	}
	defer db.Close()

	// Kiểm tra kết nối PostgreSQL
	err = db.Ping()
	if err != nil {
		log.Fatalf("Failed to ping PostgreSQL: %v", err)
	}
	log.Println("PostgreSQL is ready!")

	// API Gin router
	r := gin.Default()

	// Tạo sản phẩm
	r.POST("/product", func(c *gin.Context) {
		var requestBody struct {
			Name  string `json:"name"`
			Price int    `json:"price"`
		}
		

		if err := c.ShouldBindJSON(&requestBody); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
			return
		}
		fmt.Println("requestBody", requestBody)
		if requestBody.Name == "" || requestBody.Price <= 0 {
			errorMessage := "Invalid product data: Name is empty or Price is invalid"
			err := sendMessage("exchange2", "routingKey2", errorMessage)
			if err != nil {
				log.Printf("Failed to send error message: %v", err)
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to process invalid product"})
				return
			}
	
			// error
			c.JSON(http.StatusBadRequest, gin.H{"status": "Failed", "reason": errorMessage})
			return
		}

		// Insert product into database
		query := "INSERT INTO products (name, price) VALUES ($1, $2) RETURNING id"
		var productID int
		err := db.QueryRow(query, requestBody.Name, requestBody.Price).Scan(&productID)
		if err != nil {
			log.Printf("Failed to insert product into database: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save product"})
			return
		}
	
		// Gửi message vào RabbitMQ
		message := fmt.Sprintf("Product ID %d created: %s", productID, requestBody.Name)
		err = sendMessage("exchange1", "routingKey1", message)
		if err != nil {
			log.Printf("Failed to send message: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to process request"})
			return
		}
	
		// Respond success
		c.JSON(http.StatusOK, gin.H{
			"status": "Product created successfully",
			"id":     productID,
			"name":   requestBody.Name,
			"price":  requestBody.Price,
		})
	})

	// Chạy API
	r.Run(":8080")
}
func sendMessage(exchange, routingKey, body string) error {
	err := rabbitCh.Publish(
		exchange,   // Tên Exchange
		routingKey, // Routing Key
		false,
		false,
		amqp091.Publishing{
			ContentType: "text/plain",
			Body:        []byte(body),
		},
	)
	if err != nil {
		return err
	}
	log.Printf("Message sent to %s with routing key %s: %s", exchange, routingKey, body)
	return nil
}
func waitForRabbitMQ(host string, port string) {
	for {
		conn, err := net.Dial("tcp", net.JoinHostPort(host, port))
		if err == nil {
			conn.Close()
			log.Println("RabbitMQ is ready!")
			break
		}
		log.Println("Waiting for RabbitMQ...")
		time.Sleep(2 * time.Second)
	}
}