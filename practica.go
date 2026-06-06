package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"time"

	_ "github.com/glebarez/go-sqlite"
	"github.com/golang-jwt/jwt/v5"
)

type Libro struct {
	ID       int     `json:"id"`
	Titulo   string  `json:"titulo"`
	Autor    string  `json:"autor"`
	Precio   float64 `json:"precio"`
	Cantidad int     `json:"cantidad"`
}

var Clave = []byte("123456789")
var db *sql.DB

func main() {
	var err error
	db, err = sql.Open("sqlite", "./libreria.db")
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()
	tablaSQL := `CREATE TABLE IF NOT EXISTS libros (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		titulo TEXT,
		autor TEXT,
		precio REAL,
		cantidad INTEGER
	);`
	db.Exec(tablaSQL)

	http.HandleFunc("/login", manejadorLogin)
	http.HandleFunc("/libros", manejadorLibros)

	fmt.Println(" http://localhost:9090/libros")
	log.Fatal(http.ListenAndServe(":9090", nil))
}

func manejadorLibros(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, DELETE, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

	if r.Method == "OPTIONS" {
		w.WriteHeader(http.StatusOK)
		return
	}
	authHeader := r.Header.Get("Authorization")
	if authHeader == "" {
		http.Error(w, `{"error": "Falta el token de autorización"}`, http.StatusUnauthorized)
		return
	}

	var tokenString string
	if len(authHeader) > 7 && authHeader[:7] == "Bearer " {
		tokenString = authHeader[7:]
	} else {
		http.Error(w, `{"error": "Formato de token inválido"}`, http.StatusUnauthorized)
		return
	}
	token, err := jwt.Parse(tokenString, func(t *jwt.Token) (interface{}, error) {
		return Clave, nil
	})

	if err != nil || !token.Valid {
		http.Error(w, `{"error": "Token inválido o expirado"}`, http.StatusUnauthorized)
		return
	}

	switch r.Method {
	case "GET":
		pedirLibros(w)

	case "POST":
		meterLibro(w, r)
	case "DELETE":
		eliminarLibro(w, r)
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}
func pedirLibros(w http.ResponseWriter) {
	rows, _ := db.Query("SELECT id, titulo, autor, precio, cantidad FROM libros")
	defer rows.Close()
	lista := []Libro{}
	for rows.Next() {
		var l Libro
		rows.Scan(&l.ID, &l.Titulo, &l.Autor, &l.Precio, &l.Cantidad)
		lista = append(lista, l)
	}
	json.NewEncoder(w).Encode(lista)
}

func meterLibro(w http.ResponseWriter, r *http.Request) {
	var nuevo Libro
	json.NewDecoder(r.Body).Decode(&nuevo)
	var idExistente, cantidadActual int
	err := db.QueryRow("SELECT id, cantidad FROM libros WHERE titulo = ? AND autor = ?", nuevo.Titulo, nuevo.Autor).Scan(&idExistente, &cantidadActual)

	if err == nil {
		stmt, _ := db.Prepare("UPDATE libros SET cantidad = ? WHERE id = ?")
		stmt.Exec(cantidadActual+1, idExistente)
		nuevo.ID = idExistente
		nuevo.Cantidad = cantidadActual + 1
	} else {
		stmt, _ := db.Prepare("INSERT INTO libros (titulo, autor, precio, cantidad) VALUES (?, ?, ?, 1)")
		res, _ := stmt.Exec(nuevo.Titulo, nuevo.Autor, nuevo.Precio)
		id, _ := res.LastInsertId()
		nuevo.ID = int(id)
		nuevo.Cantidad = 1
	}
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(nuevo)
}

func eliminarLibro(w http.ResponseWriter, r *http.Request) {
	idStr := r.URL.Query().Get("id")
	id, _ := strconv.Atoi(idStr)

	var cantidadActual int
	err := db.QueryRow("SELECT cantidad FROM libros WHERE id = ?", id).Scan(&cantidadActual)

	if err == nil {
		if cantidadActual > 1 {
			stmt, _ := db.Prepare("UPDATE libros SET cantidad = ? WHERE id = ?")
			stmt.Exec(cantidadActual-1, id)
		} else {
			stmt, _ := db.Prepare("DELETE FROM libros WHERE id = ?")
			stmt.Exec(id)
		}
	}
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"status":"ok"}`))
}

func generarJWT(usuario string) (string, error) {
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"usuario": usuario,
		"exp":     time.Now().Add(time.Minute * 15).Unix(),
	})

	tokenFirmado, err := token.SignedString(Clave)
	if err != nil {
		return "", err
	}
	return tokenFirmado, nil
}

func manejadorLogin(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
	if r.Method == "OPTIONS" {
		w.WriteHeader(http.StatusOK)
		return
	}
	var credenciales struct {
		Usuario    string `json:"usuario"`
		Contrasena string `json:"contrasena"`
	}

	tokenString, err := generarJWT(credenciales.Usuario)
	if err != nil {
		http.Error(w, `{"error": "No se pudo crear el token"}`, http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"token": tokenString})
}
