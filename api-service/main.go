package main

import (
	//"database/sql"
	//"encoding/json"
	"encoding/json"
	"io"
	"log"
	//"log"
	"net/http"
	"time"
	"github.com/go-chi/cors"
	"github.com/go-chi/chi"
	"github.com/go-redis/redis/v8"
	"context"
	//"github.com/lib/pq"
	"bytes"
	//"strings"
	"github.com/IBM/sarama"
	"net"
)

type Task struct {
	ID      int    `json:"id"`
	Title   string `json:"title"`
	Content string `json:"content"`
	Done    bool   `json:"done"`
}


const (
	dbURL = "http://db-service:8081"
	kafkaBroker = "kafka:9092"
	redisURL = "redis:6379"
	kafkaTopic = "logs"
	cacheTTL = 10 * time.Minute
)
var redisPutin *redis.Client	
var ctx = context.Background()
var producer sarama.SyncProducer
func initRedis() { 
	redisPutin = redis.NewClient(&redis.Options{
		Addr: redisURL,
		Password: "",
		DB: 0,
	})
	ping, err := redisPutin.Ping(ctx).Result()
	if err != nil {
		log.Fatal(" Redis no connetted ")
	}
	log.Println("Redis connected:",ping)
}
func intiKafkaproducer() {
    config := sarama.NewConfig()
    config.Producer.Return.Successes = true
    config.Producer.RequiredAcks = sarama.WaitForAll
    config.Producer.Retry.Max = 5
    config.Net.DialTimeout = 10 * time.Second
    config.ClientID = "api-service"

    var err error
    producer, err = sarama.NewSyncProducer([]string{"kafka:9092"}, config)
    if err != nil {
        log.Fatalf("error during creating kafka producer: %v", err)
    }
    log.Println(" Kafka producer created!")
}
func sendKafkaevent(action string) {
	event := map[string]string {
		"timestamp": time.Now().Format(time.RFC3339), 
		"action":    action,
	}
	jsonEvent, err := json.Marshal(event)
	if err != nil { 
		log.Printf("serealizaation error")
		return
	}
	message := &sarama.ProducerMessage{
		Topic : kafkaTopic,
		Value : sarama.StringEncoder(jsonEvent),
	}
	partition, offset, err := producer.SendMessage(message)
	if err != nil {
		log.Printf("send message error")
		return
	}
	log.Print("Messege succseec",partition, offset)
}
func createHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
        w.WriteHeader(http.StatusMethodNotAllowed)
        return
    }
	cache, err := redisPutin.Get(ctx, "tasks_list").Result()
	if err == nil { 
		w.Header().Set("Content-Type","application/json")
		w.Write([]byte(cache))
		sendKafkaevent("task_created")
		log.Println("responce ready")
	}
	log.Println("Запрос на /api/create")
	body, _ := io.ReadAll(r.Body)
	resp, err := http.Post(dbURL+"/create","application/json",bytes.NewBuffer(body))
	if err != nil {
		return
	}
	defer resp.Body.Close()	
	sendKafkaevent("create_task")
	w.Header().Set("Content-Type","application/json")
	bodyan, err := io.ReadAll(resp.Body)
	if err != nil { 
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
	go func() {
		if err := redisPutin.Set(ctx,"task_create",bodyan,cacheTTL).Err(); err != nil {
			log.Println("request cached")
		} else { 
			log.Println("error during caching")
		}
	}()
	w.Write(bodyan)

}
func listHandler(w http.ResponseWriter, r *http.Request) {
	cache, err := redisPutin.Get(ctx, "tasks_list").Result()
	if err == nil { 
		w.Header().Set("Content-Type","application/json")
		w.Write([]byte(cache))
		sendKafkaevent("task_created")
		log.Println("responce ready")
	}
	log.Println("Запрос на /api/list")
	resp, err := http.Get(dbURL+"/list")
	if err != nil {
		return
	}
	defer resp.Body.Close()
	sendKafkaevent("list_task")
	w.Header().Set("Content-Type","application/json")
	bodyan, err := io.ReadAll(resp.Body)
	if err != nil {
		http.Error(w,err.Error(),http.StatusInternalServerError)
	}
	go func() {
		if err := redisPutin.Set(ctx,"task_list",bodyan,cacheTTL).Err(); err!= nil { 
		log.Printf("error!")
	} else { 
		log.Println("OK")
	}}()
	w.Write(bodyan)
}
func deleteHandler(w http.ResponseWriter, r *http.Request) {
	log.Println("Запрос на /api/delete/{id}")
	id := chi.URLParam(r, "id")
	body, _ := io.ReadAll(r.Body)
	req, err := http.NewRequest("DELETE",dbURL+"/delete/"+id,bytes.NewBuffer(body))
	if err != nil {
		return
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return
	}
	defer resp.Body.Close()
	sendKafkaevent("delete_task")
	w.Header().Set("Content-Type","application/json")
	bodyan, err := io.ReadAll(resp.Body)
	if err != nil {
		http.Error(w,err.Error(),http.StatusInternalServerError)
	}
	w.Write(bodyan)
	redisPutin.Del(ctx, "tasks"+id)
	redisPutin.Del(ctx, "tasks_list")
}
func doneHandler(w http.ResponseWriter, r *http.Request) {
	log.Println("Запрос на /api/done/{id}")
	id := chi.URLParam(r, "id")
	body, _ := io.ReadAll(r.Body)
	req, err := http.NewRequest("PUT",dbURL+"/done/"+id,bytes.NewBuffer(body))
	if err != nil {
		return
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return
	}
	defer resp.Body.Close()
	sendKafkaevent("done_task")
	w.Header().Set("Content-Type","application/json")
	bodyan, err := io.ReadAll(resp.Body)
	if err != nil {
		http.Error(w,err.Error(),http.StatusInternalServerError)
	}
	w.Write(bodyan)
	redisPutin.Del(ctx, "tasks"+id)
	redisPutin.Del(ctx, "tasks_list")
}
func getaskbyID(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r,"id")
	key := "task" + id
	cached, err := redisPutin.Get(ctx, key).Result()
	if err == nil { 
		w.Header().Set("Content-Type","application/json")
		w.Write([]byte(cached))
		sendKafkaevent("get")
		return 
	}
	resp, err := http.Get(dbURL+"/task/"+id)
	if err != nil {
		return
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if err := redisPutin.Set(ctx, key, body,cacheTTL).Err(); err!= nil {
		log.Fatal("cocbnt[eq]")
	}
	sendKafkaevent("get_task_by_id")
	w.Header().Set("Content-Type","application/json")
	w.Write(body)
}
func waitForKafka() {
    timeout := 2 * time.Minute
    start := time.Now()
    for {
        conn, err := net.DialTimeout("tcp", "kafka:9092", 5*time.Second)
        if err == nil {
            conn.Close()
            log.Println("Kafka ready!")
            return
        }
        if time.Since(start) > timeout {
            log.Fatal("cant connect to Kafka during 2 min")
        }
        log.Println("Waitint Kafka...")
        time.Sleep(5 * time.Second)
    }
}
func main() {
	initRedis()
	intiKafkaproducer()
	waitForKafka()
	r := chi.NewRouter()
	r.Use(cors.Handler(cors.Options{
        AllowedHeaders:   []string{"*"},
        AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
        ExposedHeaders:   []string{"Content-Type"},
        AllowCredentials: true,
    }))
	r.Options("/*", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	r.Post("/api/create", createHandler)
	r.Get("/api/list", listHandler)
	r.Get("/api/task/{id}", getaskbyID)
	r.Delete("/api/delete/{id}", deleteHandler)
	r.Put("/api/done/{id}", doneHandler)
	log.Println("Starting server on :8080")
    if err := http.ListenAndServe("0.0.0.0:8080", r); err != nil {
        log.Fatal("Server failed:", err)
    }
}
