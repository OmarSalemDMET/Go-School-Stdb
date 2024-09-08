package main

import (
	"database/sql"
	"fmt"
	"html/template"
	"io"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/gocarina/gocsv"
	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	_ "github.com/mattn/go-sqlite3"
	"golang.org/x/crypto/bcrypt"
)

type Sdata struct {
	Id   string `json:"id"`
	Day  string `json:"day"`
	Year string `json:"year"`
}

type Attendance struct {
	Id     string `json:"id"`
	Day    string `json:"day"`
	Period int    `json:"period"`
	Attend bool   `json:"attend"`
}

type Wrapper struct {
	Data []Sdata `json:"data"`
}

type Adata struct {
	Id     string `json:"id"`
	Period int    `json:"period"`
}

type Payload struct {
	Ids []Adata `json:"data"`
}

type User struct {
	username string
	password string
}

type Grade struct {
	Id    string `json:"id"`
	Day   string `json:"day"`
	Grade int    `json:"grade"`
	Mark  int    `json:"Mark"`
}

type Template struct {
	templates *template.Template
}

func (t *Template) Render(w io.Writer, name string, data interface{}, c echo.Context) error {
	return t.templates.ExecuteTemplate(w, name, data)
}



func WrapperHandler(c echo.Context) error {
	var w Wrapper
	if err := c.Bind(&w); err != nil {
		log.Println("Failed")
		return err
	}
	log.Println(w)
	return nil
}

func createAcconts(c echo.Context) error {
	db, err := sql.Open("sqlite3", "./stdb.db?_busy_timeout=50000")
	defer db.Close()
	if err != nil {
		return err
	}
	var ws []Sdata
	spin, _ := strconv.Atoi(c.FormValue("count"))
	for i := 0; i < spin; i++ {
		w := &Sdata{
			Id:   fmt.Sprint(uuid.New().ID()),
			Year: c.FormValue("year"),
			Day:  c.FormValue("day"),
		}

		ws = append(ws, *w)

		query := "INSERT INTO student VALUES(?, ?, ?)"

		_, err := db.Exec(query,
			w.Id,
			w.Day,
			w.Year)
		if err != nil {
			return err
		}

	}
	file, err := os.Create("data.csv")
	if err != nil {
		return c.Render(200, "dberror", nil)
	}
	defer file.Close()
	if err := gocsv.MarshalFile(ws, file); err != nil {
		return c.Render(200, "dberror", nil)
	}
	return c.Render(200, "dlink", nil)
}

func addAttendance(c echo.Context) error {
	db, err := sql.Open("sqlite3", "./stdb.db?_busy_timeout=5000000")
	if err != nil {
		return c.Render(200, "dberror", nil)
	}
	defer db.Close()

	tx, err := db.Begin()
	if err != nil {
		return c.Render(200, "dberror", nil)
	}
	defer func() {
		if err != nil {
			tx.Rollback() // Rollback in case of error
		} else {
			tx.Commit() // Commit the transaction if no error
		}
	}()

	ti := time.Now()
	t := fmt.Sprintf("'%d-%02d-%02d'", ti.Year(), int(ti.Month()), ti.Day())
	var p Payload

	if err := c.Bind(&p); err != nil {
		return c.Render(200, "dberror", nil)
	}

	log.Println("Payload:", p)

	for _, e := range p.Ids {
		i := e.Period
		_, err = tx.Exec("INSERT INTO attendance VALUES(?, ?, ?, ?)", e.Id, t, i, 1)
		if err != nil {
			log.Println("Error inserting attendance:", err)
			return c.Render(200, "dberror", nil)
		}
	}

	td := strings.ToLower(time.Now().Weekday().String())
	query := `
		SELECT * 
		FROM student s 
		WHERE NOT EXISTS (
			SELECT 1 
			FROM attendance a 
			WHERE a.id = s.id AND a.day = ?
		) AND s.day = ?;
	`

	rows, err := tx.Query(query, t, td)
	if err != nil {
		log.Println("Error executing query:", err)
		return c.Render(200, "dberror", nil)
	}
	defer rows.Close()

	for rows.Next() {
		var s Sdata
		if err := rows.Scan(&s.Id, &s.Day, &s.Year); err != nil {
			log.Println("Error scanning row:", err)
			return c.Render(200, "dberror", nil)
		}

		log.Println("Student not marked present:", s)
		pr := p.Ids[0].Period
		_, err = tx.Exec("INSERT INTO attendance VALUES(?, ?, ?, ?)", s.Id, t, pr, 0)
		if err != nil {
			log.Println("Error inserting absent student:", err)
			return c.Render(200, "dberror", nil)
		}
	}

	return c.Render(200, "badLogin", nil)
}
func login(c echo.Context) error {
	username := c.FormValue("username")
	password := c.FormValue("password")
	var s User
	db, err := sql.Open("sqlite3", "./stdb.db")
	if err != nil {
		return c.Render(200, "dberror", nil)
	}
	defer db.Close()
	rows, err := db.Query("SELECT username, password FROM Users WHERE username = ?", username)
	if err != nil {
		log.Println("Error executing query:", err)
		return c.Render(200, "dberror", nil)
	}
	defer rows.Close()

	for rows.Next() {
		if err := rows.Scan(&s.username, &s.password); err != nil {
			return c.Render(200, "dberror", nil)
		}
	}
	err = bcrypt.CompareHashAndPassword([]byte(s.password), []byte(password))
	if err != nil {
		return c.Render(200, "badLogin", nil)
	}

	return c.Render(200, "A_options", nil)
}

func getStudentData(c echo.Context) error {
	id := c.FormValue("id")
	db, err := sql.Open("sqlite3", "./stdb.db")
	if err != nil {
		return c.Render(200, "dberror", nil)
	}
	query := "SELECT * FROM attendance WHERE id = ?"
	rows, _ := db.Query(query, id)
	defer rows.Close()
	var a []Attendance
	for rows.Next() {
		var as Attendance
		if err3 := rows.Scan(&as.Id, &as.Day, &as.Period, &as.Attend); err3 != nil {
			log.Println(err3)
			return c.Render(200, "eMsg", nil)
		}
		log.Println(as.Attend)
		a = append(a, as)
	}

	return c.Render(200, "Attendance", a)
}

func getGrading(c echo.Context) error {
	db, err := sql.Open("sqlite3", "./stdb.db?_busy_timeout=50000")
	if err != nil {
		log.Println(err)
		return c.Render(200, "dberror", nil)
	}
	defer db.Close()
	log.Println(db)
	year := c.FormValue("year")
	day := c.FormValue("day")
	rows, err := db.Query("SELECT * FROM student WHERE year = ? AND day = ? ", year, day)
	log.Println(rows)
	if err != nil {
		log.Println(err)
		return c.Render(200, "dberror", nil)
	}
	var sarr []Sdata
	defer rows.Close()
	for rows.Next() {
		var s Sdata
		if err := rows.Scan(&s.Id, &s.Day, &s.Year); err != nil {
			log.Println(err)
			return c.Render(200, "dberror", nil)
		}
		log.Println(s)
		sarr = append(sarr, s)
	}

	return c.Render(200, "grades", sarr)
}

func gradeStudents(c echo.Context) error {
	db, err := sql.Open("sqlite3", "./stdb.db?_busy_timeout=50000")
	if err != nil {
		log.Println(err)
		return c.Render(200, "dberror", nil)
	}
	defer db.Close()
	year := c.FormValue("year")
	day := c.FormValue("day")
	tn := time.Now()
	t := fmt.Sprintf("%d-%02d-%02d",
		tn.Year(),
		int(tn.Month()),
		tn.Day())
	rows, err := db.Query("SELECT * FROM student s WHERE s.year = ? AND s.day = ? ", year, day)
	if err != nil {
		log.Println(err)
		return c.Render(200, "dberror", nil)
	}
	mark, err := strconv.Atoi(c.FormValue("mark"))
	if err != nil {
		c.Render(200, "dberror", nil)
	}
	defer rows.Close()
	var gs []Grade
	for rows.Next() {
		var s Sdata
		if err := rows.Scan(&s.Id, &s.Day, &s.Year); err != nil {
			log.Println(err)
			return c.Render(200, "dberror", nil)
		}
		gss := Grade{s.Id, t, 0, mark}
		gs = append(gs, gss)
	}
	for i := 0; i < len(gs); i++ {
		gs[i].Grade, err = strconv.Atoi(c.FormValue(gs[i].Id))
		if err != nil {
			log.Println(err)
			c.Render(200, "eMsg", nil)
		}
		sta, err := db.Prepare("INSERT INTO gradeData VALUES (?,?,?,?)")
		if err != nil {
			log.Println(err)
			c.Render(200, "dberror", nil)
		}
		sta.Exec(gs[i].Id, gs[i].Day, gs[i].Grade, gs[i].Mark)
	}
	return c.Render(200, "sMsgs", nil)
}

func getStudentMark(c echo.Context) error {
	id := c.FormValue("id")
	db, err := sql.Open("sqlite3", "./stdb.db")
	if err != nil {
		log.Println(err)
		c.Render(200, "dberror", nil)
	}
	rows, err := db.Query("SELECT * FROM gradeData WHERE id = ?", id)
	if err != nil {
		log.Println(err)
		c.Render(200, "dberror", nil)
	}
	defer rows.Close()
	var s []Grade
	for rows.Next() {
		var g Grade
		if err := rows.Scan(&g.Id, &g.Day, &g.Grade, &g.Mark); err != nil {
			log.Println(err)
			c.Render(200, "dberror", nil)
		}

		s = append(s, g)
	}
	return c.Render(200, "StudentGrades", s)
}

func getGradeLogs(c echo.Context) error {
	year := c.FormValue("year")
	day := c.FormValue("day")
	db, err := sql.Open("sqlite3", "./stdb.db?_busy_timeout=900000")
	if err != nil {
		log.Println(err)
		return c.Render(200, "dberror", nil)
	}
	defer db.Close()
	rows, err := db.Query(`SELECT g.id, g.day, g.grade, g.mark FROM gradeData g JOIN student s ON s.id = g.id WHERE s.year = ? AND s.day = ?`, year, day)
	if err != nil {
		log.Println(err)
		return c.Render(200, "dberror", nil)
	}
	defer rows.Close()
	var s []Grade
	for rows.Next() {
		var g Grade
		if err := rows.Scan(&g.Id, &g.Day, &g.Grade, &g.Mark); err != nil {
			log.Println(err)
			c.Render(200, "dberror", nil)
		}

		s = append(s, g)

	}

	file, err := os.Create("grades.csv")
	if err != nil {
		return c.Render(200, "dberror", nil)
	}
	defer file.Close()
	if err := gocsv.MarshalFile(s, file); err != nil {
		return c.Render(200, "dberror", nil)
	}

	return c.Render(200, "glink", nil)
}

func main() {
	db, err := sql.Open("sqlite3", "./stdb.db")
	if err != nil {
		log.Println(err)
	}
	defer db.Close()

	statement, _ := db.Prepare("CREATE TABLE IF NOT EXISTS student (id TEXT PRIMARY KEY, day TEXT, year INTEGER)")
	statement.Exec()
	statement, _ = db.Prepare("CREATE TABLE IF NOT EXISTS attendance (id TEXT, day TEXT, period INTEGER, attned BOOLEAN)")
	statement.Exec()

	statement, _ = db.Prepare("CREATE TABLE IF NOT EXISTS gradeData(id TEXT, day TEXT, grade INTEGER, mark Int)")
	statement.Exec()
	statement, _ = db.Prepare("CREATE TABLE IF NOT EXISTS Users(username TEXT PRIMARY KEY, password TEXT )")
	statement.Exec()
	e := echo.New()
	e.Use(middleware.Logger())
	e.Use(middleware.Static("/static"))
	e.Static("/css", "./css")
	e.Static("/js", "./Javascript")
	t := &Template{
		templates: template.Must(template.ParseGlob("./html/*.html")),
	}
	e.Renderer = t

	e.POST("/attend", WrapperHandler)
	e.POST("/createAccounts", createAcconts)
	e.POST("/addAttendance", addAttendance)
	e.POST("/login", login)
	e.POST("/getStudentData", getStudentData)
	e.POST("/gradingTable", getGrading)
	e.POST("/gradeStudents", gradeStudents)
	e.POST("/getGrades", getStudentMark)
	e.POST("/creatGradeLog", getGradeLogs)
	e.GET("/attendance", func(c echo.Context) error {
		return c.Render(200, "qrCode", nil)
	})
	e.GET("/loginPage", func(c echo.Context) error {
		return c.Render(200, "index", nil)
	})
	e.GET("/accounts", func(c echo.Context) error {
		return c.Render(200, "Panal", nil)
	})
	e.GET("/students", func(c echo.Context) error {
		return c.Render(200, "getData", nil)
	})
	e.GET("/getCsv", func(c echo.Context) error {
		return c.File("data.csv")
	})
	e.GET("/getGradesCsv", func(c echo.Context) error {
		return c.File("grades.csv")
	})
	e.GET("/qrAttend", func(c echo.Context) error {
		return c.Render(200, "qrCode", nil)
	})
	e.GET("/gradePanal", func(c echo.Context) error {
		return c.Render(200, "classroom", nil)
	})
	e.GET("/StudentsMarks", func(c echo.Context) error {
		return c.Render(200, "GradePage", nil)
	})
	e.GET("/gradesCsvs", func(c echo.Context) error {
		return c.Render(200, "classroom2", nil)
	})

	e.Logger.Fatal(e.Start(":8080"))
}
