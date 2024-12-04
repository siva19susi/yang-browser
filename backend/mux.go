package main

import (
	"archive/zip"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gorilla/mux"
)

const yangFolder = "../uploads/"

func (s *srv) logMiddleware(next http.Handler) http.Handler {
	const corHeader = "Access-Control-Allow-Origin"

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		s.logger.Printf("REQUEST: %s %s %s", r.RemoteAddr, r.Method, r.URL)

		defer func() {
			s.logger.Printf("RESPONSE: %s %s %s completed in %v", r.RemoteAddr, r.Method, r.URL, time.Since(start))
		}()

		// Set CORS header
		w.Header().Set(corHeader, "*")

		next.ServeHTTP(w, r)
	})
}

// WRITE RESPONSE PLAINTEXT
func writeResponse(w http.ResponseWriter, status string, msg string) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	if status == "error" && msg != "" {
		http.Error(w, msg, http.StatusInternalServerError)
	} else if status == "success" {
		w.Write([]byte(msg))
	} else {
		http.Error(w, "unknown error", http.StatusInternalServerError)
	}
}

// WRITE RESPONSE JSON
func writeJsonResponse(w http.ResponseWriter, b []byte) {
	w.Header().Set("Content-Type", "application/json")
	w.Write(b)
}

// Helper function to handle file creation and writing
func saveFile(file io.Reader, filepath string) error {
	out, err := os.Create(filepath)
	if err != nil {
		return fmt.Errorf("file creation failed: %v", err)
	}
	defer out.Close()

	_, err = io.Copy(out, file)
	if err != nil {
		return fmt.Errorf("copying file content failed: %v", err)
	}

	return nil
}

// RAISE ERRORS
func (s *srv) raiseError(msg string, err error, w http.ResponseWriter) {
	if err != nil {
		s.logger.Printf(msg, err)
		writeResponse(w, "error", fmt.Sprintf("%s / %v", msg, err))
	} else {
		writeResponse(w, "error", msg)
	}
}

// BACKEND CONNECTION VERIFICATION
func connectionOk(w http.ResponseWriter, r *http.Request) {
	writeResponse(w, "success", "Backend active")
}

// UPLOAD YANG REPO ZIP
func (s *srv) upload(w http.ResponseWriter, r *http.Request) {
	r.ParseMultipartForm(10 << 20)

	file, handler, err := r.FormFile("file")
	if err != nil {
		s.raiseError("retrieving zip file failed", err, w)
		return
	}
	defer file.Close()

	zipPath := yangFolder + handler.Filename
	if err := saveFile(file, zipPath); err != nil {
		s.raiseError(fmt.Sprintf("saving zip file failed %s", handler.Filename), err, w)
		return
	}

	if err := extractYangFolder(handler.Filename); err != nil {
		s.raiseError(fmt.Sprintf("extracting yang files from %s failed", handler.Filename), err, w)
		return
	}

	writeResponse(w, "success", "Repo uploaded")
}

// UNZIP YANG REPO
func extractYangFolder(filename string) error {
	zipPath := yangFolder + filename
	r, err := zip.OpenReader(zipPath)
	if err != nil {
		return fmt.Errorf("reading zip file failed: %v", err)
	}
	defer r.Close()

	basename := strings.TrimSuffix(filepath.Base(filename), filepath.Ext(filename))
	destFolder := filepath.Join(yangFolder + basename)

	if err := os.MkdirAll(destFolder, os.ModePerm); err != nil {
		return fmt.Errorf("creating repo folder failed: %v", err)
	}

	yangFileCount := 0
	for _, f := range r.File {
		if strings.HasSuffix(f.Name, ".yang") {
			yangFileCount++
			fpath := filepath.Join(destFolder, filepath.Base(f.Name))

			outFile, err := os.OpenFile(fpath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, f.Mode())
			if err != nil {
				return fmt.Errorf("setting yang file write permission failed: %v", err)
			}
			defer outFile.Close()

			rc, err := f.Open()
			if err != nil {
				return fmt.Errorf("opening yang file failed: %v", err)
			}
			defer rc.Close()

			_, err = io.Copy(outFile, rc)
			if err != nil {
				return fmt.Errorf("copying file content failed: %v", err)
			}
		}
	}

	return os.Remove(zipPath)
}

// UPLOAD FILE
func (s *srv) uploadFile(w http.ResponseWriter, r *http.Request) {
	basename, ok := mux.Vars(r)["basename"]

	r.ParseMultipartForm(10 << 20)
	file, handler, err := r.FormFile("file")
	if err != nil {
		s.raiseError("retrieving .yang file failed", err, w)
		return
	}
	defer file.Close()

	folderPath := yangFolder
	if ok {
		folderPath = yangFolder + basename + "/"
	}

	if _, err := os.Stat(folderPath); os.IsNotExist(err) {
		s.raiseError(fmt.Sprintf("repo (%s) does not exist", basename), err, w)
		return
	}

	filePath := folderPath + handler.Filename
	if err := saveFile(file, filePath); err != nil {
		s.raiseError(fmt.Sprintf("saving file %s failed", handler.Filename), err, w)
		return
	}

	writeResponse(w, "success", "File uploaded")
}

// LIST KIND
func (s *srv) list(w http.ResponseWriter, r *http.Request) {
	kind := mux.Vars(r)["kind"]

	type ListResponse struct {
		Name  string   `json:"name"`
		Files []string `json:"files,omitempty"`
	}

	var f []ListResponse

	if kind == "local" {
		dirEntries, err := os.ReadDir(yangFolder)
		if err != nil {
			s.raiseError("failed to read local uploads directory", err, w)
			return
		}

		for _, entry := range dirEntries {
			if entry.IsDir() {
				folderName := entry.Name()
				repoEntries, err := os.ReadDir(yangFolder + folderName + "/")
				if err != nil {
					s.raiseError("reading local yang repos failed", err, w)
					return
				}

				var fEntry ListResponse
				fEntry.Name = folderName
				for _, entry := range repoEntries {
					if !entry.IsDir() {
						fEntry.Files = append(fEntry.Files, entry.Name())
					}
				}
				f = append(f, fEntry)
			}
		}

		var fEntry ListResponse
		fEntry.Name = ""
		for _, entry := range dirEntries {
			if !entry.IsDir() {
				yangFile := entry.Name()
				if strings.ToLower(filepath.Ext(yangFile)) == ".yang" {
					fEntry.Files = append(fEntry.Files, yangFile)
				}
			}
		}
		f = append(f, fEntry)

	} else if kind == "nsp" {
		intentTypeList, err := s.intentTypeSearch(0, 300)
		if err != nil {
			s.raiseError("fetching NSP intent types failed", err, w)
			return
		}

		for _, entry := range intentTypeList {
			var fEntry ListResponse
			fEntry.Name = entry
			f = append(f, fEntry)
		}
	} else {
		s.raiseError("unsupported kind", nil, w)
		return
	}

	b, err := json.MarshalIndent(f, "", "  ")
	if err != nil {
		s.raiseError("JSON creation failed", err, w)
		return
	}

	writeJsonResponse(w, b)
}

// DELETE FOLDER OR REPO
func (s *srv) delete(w http.ResponseWriter, r *http.Request) {
	basename := mux.Vars(r)["basename"]
	folderPath := yangFolder + basename

	if _, err := os.Stat(folderPath); errors.Is(err, os.ErrNotExist) {
		s.raiseError(fmt.Sprintf("%s repo does not exist", basename), err, w)
		return
	}

	if err := os.RemoveAll(folderPath); err != nil {
		s.raiseError("error during repo deletion", err, w)
		return
	}

	writeResponse(w, "success", fmt.Sprintf("Local repo (%s) deleted", basename))
}

// DELETE FILE
func (s *srv) deleteFile(w http.ResponseWriter, r *http.Request) {
	basename, ok := mux.Vars(r)["basename"]
	yangFile := mux.Vars(r)["yang"]

	filePath := yangFolder + yangFile
	if ok {
		filePath = yangFolder + basename + "/" + yangFile
	}

	if _, err := os.Stat(filePath); errors.Is(err, os.ErrNotExist) {
		s.raiseError(fmt.Sprintf("%s yang file does not exist", yangFile), err, w)
		return
	}

	if err := os.Remove(filePath); err != nil {
		s.raiseError("error during file deletion", err, w)
		return
	}

	writeResponse(w, "success", fmt.Sprintf("%s yang file deleted", yangFile))
}

// NSP CONNECT
func (s *srv) nspConnect(w http.ResponseWriter, r *http.Request) {
	if err := json.NewDecoder(r.Body).Decode(&s.nsp); err != nil {
		s.raiseError("decoding NSP connect request failed", err, w)
		return
	}

	if s.nsp.Ip == "" || s.nsp.User == "" || s.nsp.Pass == "" {
		s.raiseError("NSP credentials are missing", nil, w)
		return
	}

	if err := s.getToken(); err != nil {
		s.raiseError("error making NSP connection", err, w)
		return
	}

	go s.tokenRefreshRoutine()

	writeResponse(w, "success", "NSP connected")
}

// Token refresh routine
func (s *srv) tokenRefreshRoutine() {
	for {
		s.Lock()
		tokenExpiresIn := s.nsp.token.ExpiresIn
		s.Unlock()

		if tokenExpiresIn == 0 {
			return
		}

		time.Sleep(time.Second * time.Duration(tokenExpiresIn-10))

		s.logger.Println("[Info] NSP Access renewal initiated")
		s.Lock()
		if err := s.revokeToken(); err != nil {
			s.logger.Printf("disconnecting from NSP (%s) failed: %v", s.nsp.Ip, err)
			s.Unlock()
			return
		}
		if err := s.getToken(); err != nil {
			s.logger.Printf("reconnecting to NSP (%s) failed: %v", s.nsp.Ip, err)
			s.Unlock()
			return
		}
		s.logger.Println("[Info] NSP Access renewed")
		s.Unlock()
	}
}

// NSP DISCONNECT
func (s *srv) nspDisconnect(w http.ResponseWriter, r *http.Request) {
	if s.nsp.Ip == "" {
		s.raiseError("NSP is not connected", nil, w)
		return
	}

	if err := s.revokeToken(); err != nil {
		s.raiseError(fmt.Sprintf("disconnecting from NSP (%s) failed", s.nsp.Ip), err, w)
		return
	}

	writeResponse(w, "success", "NSP disconnected")
}

// NSP IS CONNECTED
func (s *srv) nspIsConnected(w http.ResponseWriter, r *http.Request) {
	if s.nsp.token.AccessToken == "" {
		s.raiseError("NSP is not connected", nil, w)
		return
	}

	type NspAccessExport struct {
		Ip   string `json:"ip"`
		User string `json:"user"`
	}

	nspExport := NspAccessExport{
		Ip:   s.nsp.Ip,
		User: s.nsp.User,
	}

	response, err := json.MarshalIndent(nspExport, "", "  ")
	if err != nil {
		s.raiseError("JSON creation failed", err, w)
		return
	}

	writeJsonResponse(w, response)
}

// NSP MODULES
func (s *srv) getNspModules(w http.ResponseWriter, r *http.Request) {
	module, moduleProvided := mux.Vars(r)["module"]

	if moduleProvided {
		response, err := s.fetchYangDefinition(module)
		if err != nil {
			s.raiseError("error fetching YANG module definition", err, w)
			return
		}

		writeJsonResponse(w, response)
	} else {
		modules, err := s.fetchModules()
		if err != nil {
			s.raiseError("error fetching YANG modules", err, w)
			return
		}

		response, err := json.MarshalIndent(modules, "", "  ")
		if err != nil {
			s.raiseError("error creating JSON", err, w)
			return
		}
		writeJsonResponse(w, response)
	}
}
