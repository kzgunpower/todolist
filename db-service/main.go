package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"time"

	//"fmt"
	"log"
	"net/http"
	"strconv"

	"github.com/go-chi/chi"
	"github.com/go-chi/cors"
	_ "github.com/lib/pq"
)

type Task struct {
	ID      int    `json:"id"`
	Title   string `json:"title"`
	Content string `json:"content"`
	Done    bool   `json:"done"`
}

var db *sql.DB

func dbcreateHandler(w http.ResponseWriter, r *http.Request) {
	var task Task
	if err := json.NewDecoder(r.Body).Decode(&task); err != nil {
		log.Printf("Ошибка декодирования тела запроса: %v", err)
		http.Error(w, "Неверный формат данных", http.StatusBadRequest)
		return
	}

	err := db.QueryRow(`
		INSERT INTO tasks (title, content, done)
		VALUES ($1, $2, $3)
		RETURNING id
	`, task.Title, task.Content, task.Done).Scan(&task.ID)
	if err != nil {
		log.Printf("Ошибка при создании задачи: %v", err)
		http.Error(w, "Ошибка сервера", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(task)
}

func dblistHandler(w http.ResponseWriter, r *http.Request) {
	rows, err := db.Query("SELECT id, title, content, done FROM tasks")
	if err != nil {
		log.Printf("Ошибка получения списка задач: %v", err)
		http.Error(w, "Ошибка сервера", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var tasks []Task
	for rows.Next() {
		var t Task
		if err := rows.Scan(&t.ID, &t.Title, &t.Content, &t.Done); err != nil {
			log.Printf("Ошибка сканирования строки: %v", err)
			http.Error(w, "Ошибка сервера", http.StatusInternalServerError)
			return
		}
		tasks = append(tasks, t)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(tasks)
}

func dbdeleteHandler(w http.ResponseWriter, r *http.Request) {
	ctx,cancel := context.WithTimeout(r.Context(),5*time.Second)
	defer cancel()
	idStr := chi.URLParam(r, "id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		log.Printf("Неверный ID задачи: %v", err)
		http.Error(w, "Неверный ID задачи", http.StatusBadRequest)
		return
	}
	tx, err := db.BeginTx(ctx,nil)
	if err != nil { 
		log.Println("error begin tx")
	}
	defer tx.Rollback()
	res, err := tx.ExecContext(ctx,`DELETE $id FROM tasks`,id,)
	if err != nil { 
		log.Fatal("cant make tx")
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO task_audit (task_id, action)
         VALUES ($1, 'TASK_DELETED')`,
        id,)
	if err != nil { 
		log.Println("error")
	}
	rowsAffected, _ := res.RowsAffected()
	if rowsAffected == 0 {
		http.Error(w, "Задача не найдена", http.StatusNotFound)
		return
	}
	if err := tx.Commit(); err != nil { 
		log.Println("Error by commity: $v", err)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status": "success",
		"id":     id,
	})
}

func dbdoneHandler(w http.ResponseWriter, r *http.Request) {
	ctx , cancel := context.WithTimeout(r.Context(), 5 * time.Second)
	defer cancel()
	idStr := chi.URLParam(r, "id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		log.Printf("Неверный ID задачи: %v", err)
		http.Error(w, "Неверный ID задачи", http.StatusBadRequest)
		return
	}
	tx, err := db.BeginTx(ctx, nil) 
	if err != nil { 
		log.Println("не могу начать транзакцию")
	}
	defer tx.Rollback()
	res, err := db.ExecContext(ctx ,`UPDATE tasks SET done = true WHERE id = $id AND done = false`,id,)
	if err != nil { 
		log.Println("ошибка внесения изменений")
	}
	_, err = db.ExecContext(ctx,`INSERT INTO task_audit (task_id, action)
         VALUES ($1, 'TASK_UPDATED')`,id,)
	if err != nil{ 
		log.Println("cant write to audit table")
	}
	
	rowsAffected, _ := res.RowsAffected()
	if rowsAffected == 0 {
		http.Error(w, "Задача не найдена или уже выполнена", http.StatusNotFound)
		return
	}
	if err := tx.Commit(); err != nil { 
		log.Println("Error by commity: $v", err)
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status": "success",
		"id":     id,
	})
}

func getaskbyIDDB(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		log.Printf("Неверный ID задачи: %v", err)
		http.Error(w, "Неверный ID задачи", http.StatusBadRequest)
		return
	}

	var t Task
	err = db.QueryRow(
		`SELECT id, title, content, done FROM tasks WHERE id = $1`,
		id,
	).Scan(&t.ID, &t.Title, &t.Content, &t.Done)

	if err != nil {
		if err == sql.ErrNoRows {
			http.Error(w, "Задача не найдена", http.StatusNotFound)
		} else {
			log.Printf("Ошибка получения задачи: %v", err)
			http.Error(w, "Ошибка сервера", http.StatusInternalServerError)
		}
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(t)
}

func main() {
	connStr := "host=postgres user=postgres dbname=postgres sslmode=disable password=14881488"
	var err error
	db, err = sql.Open("postgres", connStr)
	if err != nil {
		log.Fatal("Ошибка подключения к БД:", err)
	}
	defer db.Close()

	if err = db.Ping(); err != nil {
		log.Fatal("Ошибка проверки подключения к БД:", err)
	}

	// Создание таблицы, если она не существует
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS tasks (
			id SERIAL PRIMARY KEY,
			title TEXT NOT NULL,
			content TEXT NOT NULL,
			done BOOLEAN DEFAULT FALSE
		)
	`)
	if err != nil {
		log.Fatal("Ошибка создания таблицы:", err)
	}

	r := chi.NewRouter()
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   []string{"*"},
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE"},
		AllowedHeaders:   []string{"Content-Type"},
		AllowCredentials: true,
	}))

	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			log.Printf("Получен %s запрос на %s", r.Method, r.URL.Path)
			next.ServeHTTP(w, r)
		})
	})

	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})
	r.Post("/create", dbcreateHandler)
	r.Get("/list", dblistHandler)
	r.Get("/task/{id}", getaskbyIDDB)
	r.Delete("/delete/{id}", dbdeleteHandler)
	r.Put("/done/{id}", dbdoneHandler)

	log.Println("Сервис БД запущен на :8081")
	if err := http.ListenAndServe("0.0.0.0:8081", r); err != nil {
		log.Fatal("Ошибка сервера:", err)
	}
}
/*package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"

	"github.com/go-chi/chi"
	"github.com/go-chi/cors"
	_ "github.com/lib/pq"
)

type Task struct {
	ID      int    `json:"id"`
	Title   string `json:"title"`
	Content string `json:"content"`
	Done    bool   `json:"done"`
}

var db *sql.DB

func dbcreateHandler(w http.ResponseWriter, r *http.Request) {
	var task Task 
	if err := json.NewDecoder(r.Body).Decode(&task); err != nil {
		return
	}
	err:= db.QueryRow(`
		INSERT INTO tasks (title, content, done)
		VALUES ($1, $2, $3)
		RETURNING id
	`, task.Title, task.Content, task.Done).Scan(&task.ID)
	if err!= nil {
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(task)
}
func dblistHandler(w http.ResponseWriter, r *http.Request) {
	rows, err := db.Query("SELECT id, title, content, done FROM tasks")
	if err !=nil {
		return
	}
	defer rows.Close()
	var tasks []Task
	for rows.Next() {
		var t Task
		if err := rows.Scan(&t.ID, &t.Title, &t.Content, &t.Done); err != nil{
			return 
		}
	tasks = append(tasks, t)
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(tasks)
}
func dbdeleteHandler(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := strconv.Atoi(idStr)
	if err !=nil {
		return
	}
	result, err := db.Exec("DELETE FROM tasks WHERE id = $1",id)
	if err != nil {
		return
	}
	rows, err:= result.RowsAffected()
	if err!=nil  {
		fmt.Print(rows)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
        "status": "success",
        "id":   id,
    })
	
}
func dbdoneHandler(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		http.Error(w, "Invalid task ID", http.StatusBadRequest)
		return
	}
	result, err := db.Exec(`UPDATE tasks SET done = true WHERE id = $1 AND done = false`, id)
	if err != nil {
		return
	}
	rows, err:= result.RowsAffected()
	if err!= nil {
		log.Fatal("ect nice",rows)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
        "status": "success",
        "id":   id,
    })
	
}
func getaskbyIDDB(w http.ResponseWriter, r *http.Request) { 
	idStr := chi.URLParam(r, "id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		http.Error(w, "Invalid task ID", http.StatusBadRequest)
		return
	}
	var t Task
	err = db.QueryRow(`SELECT id,title,content,done FROM tasks WHERE id=$1`,id).Scan(&t.ID, &t.Title, &t.Content, &t.Done)
	if err!= nil {
		fmt.Println("ошибка получения запроса")
		return
	}
}
func main() {
	connStr := "host=postgres user=postgres dbname=postgres sslmode=disable password=14881488"
	var err error
	db, err = sql.Open("postgres", connStr)
	if err != nil {
		log.Fatal("DB connection error:", err)
	}
	defer db.Close()

	
	err = db.Ping()
	if err != nil {
		log.Fatal("DB ping error:", err)
	}

	r := chi.NewRouter()

	
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   []string{"*"},
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE"},
		AllowedHeaders:   []string{"Content-Type"},
		AllowCredentials: true,
	}))
	r.Use(func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            log.Printf("Received %s request to %s", r.Method, r.URL.Path)
            next.ServeHTTP(w, r)
        })
    })
	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
        w.WriteHeader(http.StatusOK)
        w.Write([]byte("OK"))
    })
	r.Post("/create", dbcreateHandler)
	r.Get("/list", dblistHandler)
	r.Get("/task/{id}", getaskbyIDDB)
	r.Delete("/delete/{id}", dbdeleteHandler)
	r.Put("/done/{id}", dbdoneHandler)

	log.Println("Starting DB service on :8081")
	err = http.ListenAndServe("0.0.0.0:8081", r)
	if err != nil {
		log.Fatal("Server failed:", err)
	}
*/

