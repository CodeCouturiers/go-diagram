package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"go/ast"
	"log"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"sync"
	"syscall"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/gorilla/websocket"

	"goDiagram/parse"
)

const (
	writeWait      = 10 * time.Second
	pongWait       = 60 * time.Second
	pingPeriod     = (pongWait * 9) / 10
	maxMessageSize = 512 * 1024 // 512 KB
)

type ClearLayoutMessage struct {
	ClearLayout bool `json:"clearLayout"`
}

type Config struct {
	Addr              string `json:"addr"`
	DirName           string `json:"dirName"`
	DebounceInterval  string `json:"debounceInterval"`
	ConfigCheckPeriod string `json:"configCheckPeriod"`
}

var (
	upgrader = websocket.Upgrader{
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
		CheckOrigin: func(r *http.Request) bool {
			return true
		},
	}

	clients   = make(map[*Connection]bool)
	clientsMu sync.Mutex
	broadcast = make(chan interface{})

	config     Config
	configPath = "config.json"
	pkgs       map[string]*ast.Package
	pkgsMu     sync.RWMutex

	lastModTime      time.Time
	lastClientStruct *parse.ClientStruct
	fileMutex        sync.RWMutex

	server *http.Server

	debounceIntervalDuration  time.Duration
	configCheckPeriodDuration time.Duration
)

type ClientError struct {
	Error string `json:"error"`
}

type Connection struct {
	ws   *websocket.Conn
	send chan interface{}
}

func (c *Connection) reader(ctx context.Context) {
	defer func() {
		clientsMu.Lock()
		delete(clients, c)
		clientsMu.Unlock()
		err := c.ws.Close()
		if err != nil {
			return
		}
	}()
	c.ws.SetReadLimit(maxMessageSize)
	err := c.ws.SetReadDeadline(time.Now().Add(pongWait))
	if err != nil {
		log.Printf("Error setting read deadline: %v", err)
		return
	}
	c.ws.SetPongHandler(func(string) error {
		err := c.ws.SetReadDeadline(time.Now().Add(pongWait))
		if err != nil {
			return err
		}
		return nil
	})

	for {
		select {
		case <-ctx.Done():
			return
		default:
			var message json.RawMessage
			if err := c.ws.ReadJSON(&message); err != nil {
				if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
					log.Printf("Error reading JSON from client %s: %v", c.ws.RemoteAddr(), err)
				}
				return
			}

			var clientStruct parse.ClientStruct
			if err := json.Unmarshal(message, &clientStruct); err != nil {
				log.Printf("Error unmarshaling JSON from client %s: %v", c.ws.RemoteAddr(), err)
				c.send <- ClientError{Error: err.Error()}
				continue
			}

			pkgsMu.Lock()
			err := parse.WriteClientPackages(pkgs, clientStruct.Packages)
			pkgsMu.Unlock()
			if err != nil {
				log.Printf("Error writing client packages: %v", err)
				c.send <- ClientError{Error: err.Error()}
			}
			log.Printf("Processed update from client %s", c.ws.RemoteAddr())
		}
	}
}

func (c *Connection) writer(ctx context.Context) {
	pingTicker := time.NewTicker(pingPeriod)
	defer func() {
		pingTicker.Stop()
		err := c.ws.Close()
		if err != nil {
			return
		}
	}()

	for {
		select {
		case <-ctx.Done():
			return
		case message, ok := <-c.send:
			err := c.ws.SetWriteDeadline(time.Now().Add(writeWait))
			if err != nil {
				log.Printf("Error setting write deadline: %v", err)
				return
			}
			if !ok {
				err := c.ws.WriteMessage(websocket.CloseMessage, []byte{})
				if err != nil {
					return
				}
				return
			}

			if err := c.ws.WriteJSON(message); err != nil {
				log.Printf("Error writing JSON to client %s: %v", c.ws.RemoteAddr(), err)
				return
			}
			log.Printf("Sent message to client %s", c.ws.RemoteAddr())
		case <-pingTicker.C:
			err := c.ws.SetWriteDeadline(time.Now().Add(writeWait))
			if err != nil {
				log.Printf("Error setting write deadline for ping: %v", err)
				return
			}
			if err := c.ws.WriteMessage(websocket.PingMessage, nil); err != nil {
				log.Printf("Error writing ping message: %v", err)
				return
			}
		}
	}
}

func serveWs(w http.ResponseWriter, r *http.Request) {
	ws, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println(err)
		return
	}

	conn := &Connection{
		ws:   ws,
		send: make(chan interface{}, 256),
	}

	clientsMu.Lock()
	clients[conn] = true
	clientsMu.Unlock()

	clientStruct, err := readFileIfModified()
	if err != nil {
		log.Printf("Error reading initial file: %v", err)
		conn.send <- ClientError{Error: err.Error()}
	} else if clientStruct != nil {
		conn.send <- clientStruct
	}

	ctx, cancel := context.WithCancel(context.Background())
	go conn.writer(ctx)
	go conn.reader(ctx)

	// Запускаем горутину для отслеживания закрытия соединения
	go func() {
		<-ctx.Done()
		cancel()
	}()
}
func readFileIfModified() (*parse.ClientStruct, error) {
	fileMutex.RLock()
	defer fileMutex.RUnlock()

	var latestMod time.Time
	var modifiedFiles []string
	err := filepath.Walk(config.DirName, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() && filepath.Ext(path) == ".go" {
			if info.ModTime().After(lastModTime) {
				modifiedFiles = append(modifiedFiles, path)
			}
			if info.ModTime().After(latestMod) {
				latestMod = info.ModTime()
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	if !latestMod.After(lastModTime) {
		return lastClientStruct, nil
	}

	log.Printf("Detected changes in files: %v", modifiedFiles)

	fileMutex.RUnlock()
	fileMutex.Lock()
	defer func() {
		fileMutex.Unlock()
		fileMutex.RLock()
	}()

	if !latestMod.After(lastModTime) {
		return lastClientStruct, nil
	}

	clientStruct, newPkgs, err := parse.GetStructsDirName(config.DirName)
	if err != nil {
		return nil, err
	}

	// 删除重复的包、结构和方法
	removeDuplicates(clientStruct)

	pkgsMu.Lock()
	pkgs = newPkgs
	pkgsMu.Unlock()

	lastModTime = latestMod
	lastClientStruct = clientStruct

	log.Printf("Updated client struct with %d packages, %d edges",
		len(clientStruct.Packages), len(clientStruct.Edges))

	return clientStruct, nil
}

func removeDuplicates(clientStruct *parse.ClientStruct) {
	// 去重包
	pkgMap := make(map[string]*parse.Package)
	for _, pkg := range clientStruct.Packages {
		if _, exists := pkgMap[pkg.Name]; !exists {
			pkgMap[pkg.Name] = &pkg
		}
	}
	clientStruct.Packages = make([]parse.Package, 0, len(pkgMap))
	for _, pkg := range pkgMap {
		clientStruct.Packages = append(clientStruct.Packages, *pkg)
	}

	// 去重结构和方法
	for i := range clientStruct.Packages {
		pkg := &clientStruct.Packages[i]
		fileMap := make(map[string]*parse.File)
		for _, file := range pkg.Files {
			if _, exists := fileMap[file.Name]; !exists {
				fileMap[file.Name] = &file
			}
		}
		pkg.Files = make([]parse.File, 0, len(fileMap))
		for _, file := range fileMap {
			pkg.Files = append(pkg.Files, *file)

			// 去重结构
			structMap := make(map[string]*parse.Struct)
			for _, st := range file.Structs {
				if _, exists := structMap[st.Name]; !exists {
					structMap[st.Name] = &st
				}
			}
			file.Structs = make([]parse.Struct, 0, len(structMap))
			for _, st := range structMap {
				file.Structs = append(file.Structs, *st)

				// 去重方法
				methodMap := make(map[string]*parse.Method)
				for _, m := range st.Methods {
					key := fmt.Sprintf("%s:%s", st.Name, m.Name)
					if _, exists := methodMap[key]; !exists {
						methodMap[key] = &m
					}
				}
				st.Methods = make([]parse.Method, 0, len(methodMap))
				for _, m := range methodMap {
					st.Methods = append(st.Methods, *m)
				}
			}
		}
	}
}

func watchFiles(broadcast chan<- interface{}) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		log.Fatal(err)
	}
	defer func(watcher *fsnotify.Watcher) {
		err := watcher.Close()
		if err != nil {

		}
	}(watcher)

	var timer *time.Timer

	done := make(chan bool)
	go func() {
		for {
			select {
			case event, ok := <-watcher.Events:
				if !ok {
					return
				}
				if event.Op&(fsnotify.Write|fsnotify.Create|fsnotify.Remove) != 0 {
					log.Println("modified file:", event.Name)

					if timer != nil {
						timer.Stop()
					}
					timer = time.AfterFunc(debounceIntervalDuration, func() {
						updateAndBroadcast(broadcast)
					})
				}
			case err, ok := <-watcher.Errors:
				if !ok {
					return
				}
				log.Println("error:", err)
			}
		}
	}()

	err = filepath.Walk(config.DirName, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return watcher.Add(path)
		}
		return nil
	})
	if err != nil {
		log.Fatal(err)
	}
	<-done
}

func updateAndBroadcast(broadcast chan<- interface{}) {
	// Отправляем сообщение для очистки layout
	broadcast <- ClearLayoutMessage{ClearLayout: true}

	clientStruct, newPkgs, err := parse.GetStructsDirName(config.DirName)
	if err != nil {
		log.Printf("Error updating structure: %v", err)
		return
	}

	pkgsMu.Lock()
	defer pkgsMu.Unlock()

	if hasChanges(pkgs, newPkgs) {
		pkgs = newPkgs

		// Удаляем дублирующиеся структуры из main пакета
		mainPkg := findMainPackage(clientStruct)
		if mainPkg != nil {
			oldMainFileCount := len(mainPkg.Files)
			for i, file := range mainPkg.Files {
				if file.Name == "main.go" {
					mainPkg.Files = append(mainPkg.Files[:i], mainPkg.Files[i+1:]...)
					break
				}
			}
			newMainFileCount := len(mainPkg.Files)
			log.Printf("Removed %d duplicate main.go file(s) from main package", oldMainFileCount-newMainFileCount)
		}

		b, err := json.Marshal(clientStruct)
		if err != nil {
			log.Printf("Error marshaling ClientStruct: %v", err)
			return
		}
		log.Printf("Broadcasting updated structure: %s", b)

		broadcast <- clientStruct
		log.Printf("Broadcasted updated structure with %d packages, %d edges",
			len(clientStruct.Packages), len(clientStruct.Edges))
	} else {
		log.Println("No changes detected, skipping broadcast")
	}
}

func findMainPackage(clientStruct *parse.ClientStruct) *parse.Package {
	for i, pkg := range clientStruct.Packages {
		if pkg.Name == "main" {
			return &clientStruct.Packages[i]
		}
	}
	return nil
}
func hasChanges(oldPkgs, newPkgs map[string]*ast.Package) bool {
	if len(oldPkgs) != len(newPkgs) {
		return true
	}

	for name, oldPkg := range oldPkgs {
		newPkg, exists := newPkgs[name]
		if !exists {
			return true
		}

		if len(oldPkg.Files) != len(newPkg.Files) {
			return true
		}

		for fileName, oldFile := range oldPkg.Files {
			newFile, exists := newPkg.Files[fileName]
			if !exists {
				return true
			}

			if !compareAST(oldFile, newFile) {
				return true
			}
		}
	}

	return false
}

func compareAST(oldFile, newFile *ast.File) bool {
	oldStructs := make(map[string]*ast.StructType)
	newStructs := make(map[string]*ast.StructType)

	ast.Inspect(oldFile, func(n ast.Node) bool {
		if ts, ok := n.(*ast.TypeSpec); ok {
			if st, ok := ts.Type.(*ast.StructType); ok {
				oldStructs[ts.Name.Name] = st
			}
		}
		return true
	})

	ast.Inspect(newFile, func(n ast.Node) bool {
		if ts, ok := n.(*ast.TypeSpec); ok {
			if st, ok := ts.Type.(*ast.StructType); ok {
				newStructs[ts.Name.Name] = st
			}
		}
		return true
	})

	if len(oldStructs) != len(newStructs) {
		return false
	}

	for name, oldStruct := range oldStructs {
		newStruct, exists := newStructs[name]
		if !exists {
			return false
		}

		if !compareStructs(oldStruct, newStruct) {
			return false
		}
	}

	return true
}

func compareStructs(oldStruct, newStruct *ast.StructType) bool {
	if len(oldStruct.Fields.List) != len(newStruct.Fields.List) {
		return false
	}

	for i, oldField := range oldStruct.Fields.List {
		newField := newStruct.Fields.List[i]

		if len(oldField.Names) != len(newField.Names) {
			return false
		}

		for j, oldName := range oldField.Names {
			if oldName.Name != newField.Names[j].Name {
				return false
			}
		}

		if !compareExpr(oldField.Type, newField.Type) {
			return false
		}
	}

	return true
}

func compareExpr(oldExpr, newExpr ast.Expr) bool {
	return fmt.Sprintf("%T", oldExpr) == fmt.Sprintf("%T", newExpr) &&
		fmt.Sprint(oldExpr) == fmt.Sprint(newExpr)
}

func loadConfig() error {
	file, err := os.ReadFile(configPath)
	if err != nil {
		return fmt.Errorf("error reading config file: %w", err)
	}

	var rawConfig Config
	err = json.Unmarshal(file, &rawConfig)
	if err != nil {
		return fmt.Errorf("error parsing config file: %w", err)
	}

	debounceInterval, err := time.ParseDuration(rawConfig.DebounceInterval)
	if err != nil {
		return fmt.Errorf("invalid debounceInterval: %w", err)
	}

	configCheckPeriod, err := time.ParseDuration(rawConfig.ConfigCheckPeriod)
	if err != nil {
		return fmt.Errorf("invalid configCheckPeriod: %w", err)
	}

	config = rawConfig
	debounceIntervalDuration = debounceInterval
	configCheckPeriodDuration = configCheckPeriod

	return nil
}

func watchConfig() {
	ticker := time.NewTicker(configCheckPeriodDuration)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			newConfig := Config{}
			file, err := os.ReadFile(configPath)
			if err != nil {
				log.Printf("Error reading config file: %v", err)
				continue
			}

			err = json.Unmarshal(file, &newConfig)
			if err != nil {
				log.Printf("Error parsing config file: %v", err)
				continue
			}

			if newConfig != config {
				log.Printf("Config changed, updating...")

				oldConfig := config
				config = newConfig

				var err error
				debounceIntervalDuration, err = time.ParseDuration(config.DebounceInterval)
				if err != nil {
					log.Printf("Invalid debounceInterval: %v", err)
					continue
				}
				configCheckPeriodDuration, err = time.ParseDuration(config.ConfigCheckPeriod)
				if err != nil {
					log.Printf("Invalid configCheckPeriod: %v", err)
					continue
				}

				if oldConfig.Addr != config.Addr {
					log.Println("Server address changed, restart required")
					// Здесь можно добавить логику для перезапуска сервера, если это необходимо
				}

				if oldConfig.DirName != config.DirName {
					log.Printf("Directory changed from %s to %s, updating...", oldConfig.DirName, config.DirName)

					// Очищаем старые данные
					pkgsMu.Lock()
					pkgs = make(map[string]*ast.Package)
					pkgsMu.Unlock()

					lastModTime = time.Time{}
					lastClientStruct = nil

					// Отправляем сообщение для очистки layout
					broadcast <- ClearLayoutMessage{ClearLayout: true}

					// Обновляем структуры данных с новой директорией
					clientStruct, err := readFileIfModified()
					if err != nil {
						log.Printf("Error reading new directory: %v", err)
					} else if clientStruct != nil {
						broadcast <- clientStruct
					}
				}
			}
		}
	}
}

func main() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)

	err := loadConfig()
	if err != nil {
		log.Fatalf("Error loading configuration: %v", err)
	}

	log.Printf("Starting server with configuration: %+v", config)

	go watchFiles(broadcast)
	go watchConfig()

	go func() {
		for msg := range broadcast {
			clientsMu.Lock()
			for client := range clients {
				select {
				case client.send <- msg:
				default:
					close(client.send)
					delete(clients, client)
				}
			}
			clientsMu.Unlock()
		}
	}()

	http.Handle("/", http.FileServer(http.Dir("./app/build")))
	http.HandleFunc("/ws", serveWs)

	server = &http.Server{
		Addr:    config.Addr,
		Handler: nil,
	}

	go func() {
		log.Printf("Listening on http://localhost%s", config.Addr)
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("Error starting server: %v", err)
		}
	}()

	openBrowser(fmt.Sprintf("http://localhost%s", config.Addr))

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	<-stop

	log.Println("Shutting down server...")

	clientsMu.Lock()
	for client := range clients {
		close(client.send)
		err := client.ws.Close()
		if err != nil {
			return
		}
	}
	clientsMu.Unlock()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		log.Fatalf("Server forced to shutdown: %v", err)
	}

	log.Println("Server exiting")
}

func openBrowser(url string) {
	var err error

	switch runtime.GOOS {
	case "linux":
		err = exec.Command("xdg-open", url).Start()
	case "windows":
		err = exec.Command("rundll32", "url.dll,FileProtocolHandler", url).Start()
	case "darwin":
		err = exec.Command("open", url).Start()
	default:
		err = fmt.Errorf("unsupported platform")
	}

	if err != nil {
		log.Printf("Error opening browser: %v", err)
	}
}
