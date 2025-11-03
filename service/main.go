package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"net/http"
	"os"
	"os/signal"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	tiktoken "github.com/pkoukk/tiktoken-go"
	"golang.org/x/time/rate"
)

type TokenSavings struct {
	JSON       int     `json:"json"`
	TOON       int     `json:"toon"`
	Saved      int     `json:"saved"`
	Percentage float64 `json:"percentage"`
}

type visitor struct {
	limiter  *rate.Limiter
	lastSeen time.Time
}

var (
	visitors = make(map[string]*visitor)
	mu       sync.RWMutex
)

var (
	tokenizer     *tiktoken.Tiktoken
	tokenizerOnce sync.Once
	tokenizerErr  error
)

func initTokenizer() {
	tokenizerOnce.Do(func() {
		// Usar o200k_base (GPT-4o, GPT-5)
		tokenizer, tokenizerErr = tiktoken.GetEncoding("o200k_base")
	})
}

func getVisitor(ip string) *rate.Limiter {
	mu.Lock()
	defer mu.Unlock()

	v, exists := visitors[ip]
	if !exists {
		limiter := rate.NewLimiter(5, 10) // 5 req/seg, burst 10
		visitors[ip] = &visitor{limiter: limiter, lastSeen: time.Now()}
		return limiter
	}

	v.lastSeen = time.Now()
	return v.limiter
}

func cleanupVisitors() {
	for {
		time.Sleep(time.Minute)
		mu.Lock()
		for ip, v := range visitors {
			if time.Since(v.lastSeen) > 3*time.Minute {
				delete(visitors, ip)
			}
		}
		mu.Unlock()
	}
}

func getIP(r *http.Request) string {
	ip := r.Header.Get("X-Forwarded-For")
	if ip != "" {
		// Tomar el primer valor si hay múltiples
		if idx := strings.Index(ip, ","); idx > 0 {
			ip = strings.TrimSpace(ip[:idx])
		}
		return ip
	}
	ip = r.RemoteAddr
	if idx := strings.LastIndex(ip, ":"); idx > 0 {
		ip = ip[:idx]
	}
	return ip
}

func rateLimitMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ip := getIP(r)
		limiter := getVisitor(ip)
		if !limiter.Allow() {
			http.Error(w, "Rate limit exceeded", http.StatusTooManyRequests)
			return
		}
		next(w, r)
	}
}

func securityMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Accept")
		w.Header().Set("Access-Control-Allow-Credentials", "false")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("X-XSS-Protection", "1; mode=block")

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func recoveryMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if err := recover(); err != nil {
				log.Printf("Panic recovered: %v", err)
				http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			}
		}()
		next.ServeHTTP(w, r)
	})
}

func loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		log.Printf("%s %s %v", r.Method, r.URL.Path, time.Since(start))
	})
}

func main() {
	go cleanupVisitors()

	mux := http.NewServeMux()
	mux.Handle("/", http.FileServer(http.Dir("static")))
	mux.HandleFunc("/api/count-tokens", rateLimitMiddleware(countTokensAPI))
	mux.HandleFunc("/api/fix-json", rateLimitMiddleware(fixJSONAPI))
	mux.HandleFunc("/api/json-to-toon", rateLimitMiddleware(jsonToToonAPI))

	server := &http.Server{
		Addr:           ":8080",
		Handler:        recoveryMiddleware(loggingMiddleware(securityMiddleware(mux))),
		ReadTimeout:    10 * time.Second,
		WriteTimeout:   10 * time.Second,
		IdleTimeout:    120 * time.Second,
		MaxHeaderBytes: 1 << 20,
	}

	log.Println("Servidor iniciado en :8080")

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Error iniciando servidor: %v", err)
		}
	}()

	<-sigChan
	log.Println("Señal de apagado recibida, cerrando servidor...")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		log.Printf("Error durante el shutdown: %v", err)
	} else {
		log.Println("Servidor detenido correctamente")
	}
}

const maxPayloadSize = 1 << 20 // 1MB

func jsonToToonAPI(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")

	type request struct {
		JSON         string `json:"json"`
		Delimiter    string `json:"delimiter,omitempty"`    // ",", "\t", "|"
		LengthMarker bool   `json:"lengthMarker,omitempty"` // true/false
		Indent       int    `json:"indent,omitempty"`       // espacios de indentación
	}
	type response struct {
		Toon         string        `json:"toon,omitempty"`
		Error        string        `json:"error,omitempty"`
		Fixed        bool          `json:"fixed,omitempty"`
		Original     string        `json:"original,omitempty"`
		TokenSavings *TokenSavings `json:"tokenSavings,omitempty"`
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxPayloadSize)

	var req request
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		if err.Error() == "http: request body too large" {
			json.NewEncoder(w).Encode(response{Error: "Cuerpo de la petición demasiado grande (máximo 1MB)"})
			return
		}
		json.NewEncoder(w).Encode(response{Error: "Error de decodificación del body"})
		return
	}

	if len(req.JSON) > 500000 {
		json.NewEncoder(w).Encode(response{Error: "JSON demasiado grande (máximo 500,000 caracteres)"})
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	type result struct {
		toon         string
		tokenSavings *TokenSavings
		fixed        bool
		err          error
	}

	resultChan := make(chan result, 1)

	go func() {
		var data interface{}
		err := json.Unmarshal([]byte(req.JSON), &data)

		wasFixed := false
		if err != nil {
			fixed := tryFixJSON(req.JSON)
			if err := json.Unmarshal([]byte(fixed), &data); err != nil {
				resultChan <- result{err: fmt.Errorf("JSON inválido: %v", err)}
				return
			}
			wasFixed = true
		}

		// Crear encoder con opciones
		opts := TOONOptions{
			Delimiter:    req.Delimiter,
			LengthMarker: req.LengthMarker,
			Indent:       req.Indent,
		}
		encoder, err := NewTOONEncoderWithOptions(opts)
		if err != nil {
			resultChan <- result{err: err}
			return
		}
		toon := encoder.Encode(data)

		// Calcular tokens
		jsonTokens := countTokens(req.JSON)
		toonTokens := countTokens(toon)

		var tokenSavings *TokenSavings
		if jsonTokens > 0 && toonTokens > 0 {
			saved := jsonTokens - toonTokens
			percentage := float64(saved) / float64(jsonTokens) * 100
			tokenSavings = &TokenSavings{
				JSON:       jsonTokens,
				TOON:       toonTokens,
				Saved:      saved,
				Percentage: math.Round(percentage*100) / 100,
			}
		}

		resultChan <- result{toon: toon, tokenSavings: tokenSavings, fixed: wasFixed}
	}()

	select {
	case res := <-resultChan:
		if res.err != nil {
			json.NewEncoder(w).Encode(response{
				Error:    res.err.Error(),
				Original: req.JSON,
			})
			return
		}

		resp := response{
			Toon:         res.toon,
			TokenSavings: res.tokenSavings,
		}

		if res.fixed {
			resp.Fixed = true
			resp.Error = "JSON corregido automáticamente"
		}

		json.NewEncoder(w).Encode(resp)
	case <-ctx.Done():
		json.NewEncoder(w).Encode(response{Error: "Tiempo de procesamiento excedido"})
	}
}

// Intenta corregir errores comunes de formato JSON
func tryFixJSON(input string) string {
	s := strings.TrimSpace(input)

	// 1. Eliminar comas duplicadas
	re := regexp.MustCompile(`,\s*,+`)
	s = re.ReplaceAllString(s, ",")

	// 2. Eliminar comas antes de llaves/corchetes de cierre
	s = regexp.MustCompile(`,\s*}`).ReplaceAllString(s, "}")
	s = regexp.MustCompile(`,\s*]`).ReplaceAllString(s, "]")

	// 3. Agregar comas faltantes entre propiedades (caso: "a":1"b":2)
	s = regexp.MustCompile(`("\s*:\s*[^,}\]]+)\s*"`).ReplaceAllString(s, `$1,"`)

	// 4. Corregir llaves desbalanceadas
	openBraces := strings.Count(s, "{")
	closeBraces := strings.Count(s, "}")
	if openBraces > closeBraces {
		s += strings.Repeat("}", openBraces-closeBraces)
	} else if closeBraces > openBraces {
		s = strings.Repeat("{", closeBraces-openBraces) + s
	}

	// 5. Corregir corchetes desbalanceados
	openBrackets := strings.Count(s, "[")
	closeBrackets := strings.Count(s, "]")
	if openBrackets > closeBrackets {
		s += strings.Repeat("]", openBrackets-closeBrackets)
	} else if closeBrackets > openBrackets {
		s = strings.Repeat("[", closeBrackets-openBrackets) + s
	}

	// 6. Intentar agregar comillas faltantes a claves (muy básico)
	// Ejemplo: {name: "value"} -> {"name": "value"}
	re = regexp.MustCompile(`([{,]\s*)([a-zA-Z_][a-zA-Z0-9_]*)\s*:`)
	s = re.ReplaceAllString(s, `$1"$2":`)

	return s
}

func fixJSONAPI(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")

	type request struct {
		JSON string `json:"json"`
	}
	type response struct {
		Fixed    string   `json:"fixed,omitempty"`
		Error    string   `json:"error,omitempty"`
		Original string   `json:"original,omitempty"`
		Changes  []string `json:"changes,omitempty"`
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxPayloadSize)

	var req request
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		if err.Error() == "http: request body too large" {
			json.NewEncoder(w).Encode(response{Error: "Cuerpo de la petición demasiado grande (máximo 1MB)"})
			return
		}
		json.NewEncoder(w).Encode(response{Error: "Error de decodificación del body"})
		return
	}

	if len(req.JSON) > 500000 {
		json.NewEncoder(w).Encode(response{Error: "JSON demasiado grande (máximo 500,000 caracteres)"})
		return
	}

	original := strings.TrimSpace(req.JSON)
	fixed, changes := fixJSON(original)

	// Verificar que el JSON corregido sea válido
	var test interface{}
	if err := json.Unmarshal([]byte(fixed), &test); err != nil {
		json.NewEncoder(w).Encode(response{
			Error:    fmt.Sprintf("No se pudo corregir el JSON: %v", err),
			Original: original,
		})
		return
	}

	json.NewEncoder(w).Encode(response{
		Fixed:   fixed,
		Changes: changes,
	})
}

func fixJSON(input string) (string, []string) {
	s := strings.TrimSpace(input)
	var changes []string

	// 1. Eliminar comentarios (// y /* */)
	original := s
	re := regexp.MustCompile(`(?s)/\*.*?\*/|//.*?$`)
	s = re.ReplaceAllStringFunc(s, func(match string) string {
		changes = append(changes, fmt.Sprintf("Eliminado comentario: %s", strings.TrimSpace(match)))
		return ""
	})
	if s != original {
		s = strings.TrimSpace(s)
	}

	// 2. Eliminar comas duplicadas
	original = s
	re = regexp.MustCompile(`,\s*,+`)
	s = re.ReplaceAllStringFunc(s, func(match string) string {
		changes = append(changes, fmt.Sprintf("Eliminada coma duplicada: %s", match))
		return ","
	})

	// 3. Eliminar comas antes de llaves/corchetes de cierre
	original = s
	s = regexp.MustCompile(`,\s*}`).ReplaceAllStringFunc(s, func(match string) string {
		changes = append(changes, "Eliminada coma antes de }")
		return "}"
	})
	s = regexp.MustCompile(`,\s*]`).ReplaceAllStringFunc(s, func(match string) string {
		changes = append(changes, "Eliminada coma antes de ]")
		return "]"
	})

	// 4. Agregar comas faltantes entre propiedades
	original = s
	re = regexp.MustCompile(`("\s*:\s*[^,}\]]+)\s*("[{\[])`)
	s = re.ReplaceAllStringFunc(s, func(match string) string {
		changes = append(changes, "Agregada coma faltante entre propiedades")
		return "$1,$2"
	})

	// 5. Balancear llaves y corchetes
	openBraces := strings.Count(s, "{")
	closeBraces := strings.Count(s, "}")
	if openBraces > closeBraces {
		s += strings.Repeat("}", openBraces-closeBraces)
		changes = append(changes, fmt.Sprintf("Agregadas %d llaves de cierre", openBraces-closeBraces))
	} else if closeBraces > openBraces {
		s = strings.Repeat("{", closeBraces-openBraces) + s
		changes = append(changes, fmt.Sprintf("Agregadas %d llaves de apertura", closeBraces-openBraces))
	}

	openBrackets := strings.Count(s, "[")
	closeBrackets := strings.Count(s, "]")
	if openBrackets > closeBrackets {
		s += strings.Repeat("]", openBrackets-closeBrackets)
		changes = append(changes, fmt.Sprintf("Agregados %d corchetes de cierre", openBrackets-closeBrackets))
	} else if closeBrackets > openBrackets {
		s = strings.Repeat("[", closeBrackets-openBrackets) + s
		changes = append(changes, fmt.Sprintf("Agregados %d corchetes de apertura", closeBrackets-openBrackets))
	}

	// 6. Agregar comillas a claves sin comillas
	original = s
	re = regexp.MustCompile(`([{,]\s*)([a-zA-Z_][a-zA-Z0-9_]*)\s*:`)
	s = re.ReplaceAllStringFunc(s, func(match string) string {
		changes = append(changes, "Agregadas comillas a clave sin comillas")
		return "$1\"$2\":"
	})

	// 7. Corregir comillas simples a dobles en claves
	original = s
	re = regexp.MustCompile(`'([^']*)'(\s*:)`)
	s = re.ReplaceAllStringFunc(s, func(match string) string {
		changes = append(changes, "Convertidas comillas simples a dobles en clave")
		return "\"$1\"$2"
	})

	// 8. Corregir valores true/false/null sin comillas
	original = s
	re = regexp.MustCompile(`([{,]\s*"[^"]*"\s*:\s*)(true|false|null)([},])`)
	s = re.ReplaceAllStringFunc(s, func(match string) string {
		changes = append(changes, "Corregido valor primitivo sin comillas")
		return "$1\"$2\"$3"
	})

	return s, changes
}

type TOONOptions struct {
	Indent       int
	Delimiter    string // ",", "\t", "|"
	LengthMarker bool   // true para usar '#'
}

type TOONEncoder struct {
	indent       string
	delimiter    string
	lengthMarker string // "#" or ""
}

func NewTOONEncoder() *TOONEncoder {
	return &TOONEncoder{
		indent:       "  ", // 2 espacios
		delimiter:    ",",
		lengthMarker: "",
	}
}

func NewTOONEncoderWithOptions(opts TOONOptions) (*TOONEncoder, error) {
	indent := "  "
	if opts.Indent > 0 {
		indent = strings.Repeat(" ", opts.Indent)
	}

	delimiter := ","
	if opts.Delimiter != "" {
		if opts.Delimiter != "," && opts.Delimiter != "\t" && opts.Delimiter != "|" {
			return nil, fmt.Errorf("invalid delimiter: %q (must be ',', '\\t', or '|')", opts.Delimiter)
		}
		delimiter = opts.Delimiter
	}

	lengthMarker := ""
	if opts.LengthMarker {
		lengthMarker = "#"
	}

	return &TOONEncoder{
		indent:       indent,
		delimiter:    delimiter,
		lengthMarker: lengthMarker,
	}, nil
}

func (e *TOONEncoder) Encode(value interface{}) string {
	return e.encodeValue(value, 0)
}

const maxDepth = 100

func (e *TOONEncoder) encodeValue(value interface{}, depth int) string {
	if depth > maxDepth {
		return `"[MAX_DEPTH_EXCEEDED]"`
	}

	if value == nil {
		return "null"
	}

	switch v := value.(type) {
	case bool:
		return strconv.FormatBool(v)
	case float64:
		return e.encodeNumber(v)
	case string:
		return e.encodeString(v)
	case map[string]interface{}:
		return e.encodeObject(v, depth)
	case []interface{}:
		return e.encodeArray(v, depth)
	default:
		return fmt.Sprintf("%v", v)
	}
}

func (e *TOONEncoder) encodeNumber(n float64) string {
	if n == 0 {
		return "0"
	}

	if math.IsNaN(n) || math.IsInf(n, 0) {
		return "null"
	}

	// Manejar números muy grandes sin notación científica
	if math.Abs(n) >= 1e15 {
		return fmt.Sprintf("%.0f", n)
	}

	if n >= 1e6 || (n > 0 && n <= 1e-6) {
		return fmt.Sprintf("%.0f", n)
	}

	if n == float64(int64(n)) {
		return fmt.Sprintf("%d", int64(n))
	}

	return strconv.FormatFloat(n, 'f', -1, 64)
}

func (e *TOONEncoder) encodeString(s string) string {
	needsQuotes := false

	if s == "" {
		return `""`
	}

	if strings.TrimSpace(s) != s {
		needsQuotes = true
	}

	// CRÍTICO: Quote si contiene el delimitador ACTIVO
	if strings.Contains(s, e.delimiter) {
		needsQuotes = true
	}

	// Quote si contiene :, comillas, backslash, o control chars
	if strings.ContainsAny(s, `:"'\`) ||
		strings.Contains(s, "\n") ||
		strings.Contains(s, "\t") ||
		strings.Contains(s, "\r") {
		needsQuotes = true
	}

	lower := strings.ToLower(s)
	if lower == "true" || lower == "false" || lower == "null" {
		needsQuotes = true
	}
	if _, err := strconv.ParseFloat(s, 64); err == nil {
		needsQuotes = true
	}

	if strings.HasPrefix(s, "- ") {
		needsQuotes = true
	}

	if strings.HasPrefix(s, "[") || strings.HasPrefix(s, "{") {
		needsQuotes = true
	}

	if needsQuotes {
		escaped := strings.ReplaceAll(s, `\`, `\\`)
		escaped = strings.ReplaceAll(escaped, `"`, `\"`)
		escaped = strings.ReplaceAll(escaped, "\n", `\n`)
		escaped = strings.ReplaceAll(escaped, "\t", `\t`)
		escaped = strings.ReplaceAll(escaped, "\r", `\r`)
		return `"` + escaped + `"`
	}

	return s
}

func (e *TOONEncoder) encodeObject(obj map[string]interface{}, depth int) string {
	if len(obj) == 0 {
		return ""
	}

	// Ordenar claves para salida determinística
	keys := make([]string, 0, len(obj))
	for k := range obj {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var lines []string
	indentation := strings.Repeat(e.indent, depth)

	for _, key := range keys {
		value := obj[key]
		encodedKey := e.encodeKey(key)

		// Determinar formato según tipo de valor
		switch v := value.(type) {
		case map[string]interface{}:
			if len(v) == 0 {
				lines = append(lines, indentation+encodedKey+":")
			} else {
				lines = append(lines, indentation+encodedKey+":")
				nested := e.encodeObject(v, depth+1)
				lines = append(lines, nested)
			}

		case []interface{}:
			arrayStr := e.encodeArray(v, depth+1)
			if strings.Contains(arrayStr, "\n") {
				// Array multilínea
				lines = append(lines, indentation+encodedKey+arrayStr)
			} else {
				// Array inline
				lines = append(lines, indentation+encodedKey+arrayStr)
			}

		default:
			// Valor primitivo
			encoded := e.encodeValue(value, depth)
			lines = append(lines, indentation+encodedKey+": "+encoded)
		}
	}

	return strings.Join(lines, "\n")
}

func (e *TOONEncoder) encodeKeyWithDelimiter(key string, inArray bool) string {
	// Claves necesitan comillas si:
	// - Contienen espacios, comas, colons, comillas
	// - Contienen brackets/braces
	// - Comienzan con guión
	// - Son solo números
	// - Están vacías

	if key == "" {
		return `""`
	}

	needsQuotes := false

	if inArray {
		// En arrays, quote si contiene el delimitador activo
		if strings.Contains(key, e.delimiter) {
			needsQuotes = true
		}
		if strings.ContainsAny(key, ` :"'[]{}`) {
			needsQuotes = true
		}
	} else {
		if strings.ContainsAny(key, ` ,:"'[]{}`) {
			needsQuotes = true
		}
	}

	if strings.HasPrefix(key, "-") {
		needsQuotes = true
	}

	if _, err := strconv.ParseFloat(key, 64); err == nil {
		needsQuotes = true
	}

	if needsQuotes {
		if inArray {
			escaped := strings.ReplaceAll(key, `\`, `\\`)
			escaped = strings.ReplaceAll(escaped, `"`, `\"`)
			return `"` + escaped + `"`
		} else {
			escaped := strings.ReplaceAll(key, `"`, `\"`)
			return `"` + escaped + `"`
		}
	}

	return key
}

func (e *TOONEncoder) encodeKey(key string) string {
	return e.encodeKeyWithDelimiter(key, false)
}

// Nueva función para encodear claves en arrays tabulares
func (e *TOONEncoder) encodeKeyForArray(key string) string {
	return e.encodeKeyWithDelimiter(key, true)
}

func (e *TOONEncoder) encodeArray(arr []interface{}, depth int) string {
	length := len(arr)

	if length == 0 {
		return "[0]:"
	}

	// Verificar si es array tabular (todos objetos con mismas claves primitivas)
	if isTabular, fields := e.isTabularArray(arr); isTabular {
		return e.encodeTabularArray(arr, fields, depth)
	}

	// Verificar si todos son primitivos
	if e.allPrimitive(arr) {
		return e.encodePrimitiveArray(arr, length)
	}

	// Formato lista (fallback)
	return e.encodeListArray(arr, depth, length)
}

func (e *TOONEncoder) isTabularArray(arr []interface{}) (bool, []string) {
	if len(arr) == 0 {
		return false, nil
	}

	// Primer elemento debe ser objeto
	firstObj, ok := arr[0].(map[string]interface{})
	if !ok {
		return false, nil
	}

	// Obtener claves del primer objeto (ordenadas)
	fields := make([]string, 0, len(firstObj))
	for k := range firstObj {
		fields = append(fields, k)
	}
	sort.Strings(fields)

	// Verificar todos los elementos
	for _, item := range arr {
		obj, ok := item.(map[string]interface{})
		if !ok {
			return false, nil
		}

		// Misma cantidad de campos
		if len(obj) != len(fields) {
			return false, nil
		}

		// Mismos campos y todos primitivos
		for _, field := range fields {
			val, exists := obj[field]
			if !exists {
				return false, nil
			}

			// Verificar que sea primitivo
			switch val.(type) {
			case map[string]interface{}, []interface{}:
				return false, nil
			}
		}
	}

	return true, fields
}

func (e *TOONEncoder) encodeTabularArray(arr []interface{}, fields []string, depth int) string {
	length := len(arr)
	indentation := strings.Repeat(e.indent, depth)

	// Determinar delimitador para header
	var headerDelimiter string
	var lengthDelimiter string

	switch e.delimiter {
	case "\t":
		headerDelimiter = " "
		lengthDelimiter = " "
	case "|":
		headerDelimiter = "|"
		lengthDelimiter = "|"
	default: // comma
		headerDelimiter = ","
		lengthDelimiter = ""
	}

	// Encodear claves para el header
	encodedFields := make([]string, len(fields))
	for i, field := range fields {
		encodedFields[i] = e.encodeKeyForArray(field)
	}
	fieldList := strings.Join(encodedFields, headerDelimiter)

	header := fmt.Sprintf("[%s%d%s]{%s}:",
		e.lengthMarker,
		length,
		lengthDelimiter,
		fieldList)

	// Filas - usar fields originales
	var rows []string
	for _, item := range arr {
		obj := item.(map[string]interface{})
		var values []string

		for _, field := range fields { // Usar fields, no encodedFields
			val := obj[field]
			encoded := e.encodeValue(val, depth)
			if s, ok := val.(string); ok {
				encoded = e.encodeString(s)
			}
			values = append(values, encoded)
		}

		row := indentation + e.indent + strings.Join(values, e.delimiter)
		rows = append(rows, row)
	}

	return header + "\n" + strings.Join(rows, "\n")
}

func (e *TOONEncoder) allPrimitive(arr []interface{}) bool {
	for _, item := range arr {
		switch item.(type) {
		case map[string]interface{}, []interface{}:
			return false
		}
	}
	return true
}

func (e *TOONEncoder) encodePrimitiveArray(arr []interface{}, length int) string {
	var values []string
	for _, item := range arr {
		encoded := e.encodeValue(item, 0)
		if s, ok := item.(string); ok {
			encoded = e.encodeString(s)
		}
		values = append(values, encoded)
	}

	// Delimiter marker para header
	var delimiterMarker string
	switch e.delimiter {
	case "\t":
		delimiterMarker = " "
	case "|":
		delimiterMarker = "|"
	}

	return fmt.Sprintf("[%s%d%s]: %s",
		e.lengthMarker,
		length,
		delimiterMarker,
		strings.Join(values, e.delimiter))
}

func (e *TOONEncoder) encodeListArray(arr []interface{}, depth int, length int) string {
	indentation := strings.Repeat(e.indent, depth)

	var lines []string
	lines = append(lines, fmt.Sprintf("[%s%d]:", e.lengthMarker, length))

	for _, item := range arr {
		switch v := item.(type) {
		case map[string]interface{}:
			// Objeto en lista
			if len(v) == 0 {
				lines = append(lines, indentation+e.indent+"- ")
			} else {
				// Primera propiedad en línea del guión
				keys := make([]string, 0, len(v))
				for k := range v {
					keys = append(keys, k)
				}
				sort.Strings(keys)

				firstKey := keys[0]
				firstVal := e.encodeValue(v[firstKey], depth+1)
				lines = append(lines, indentation+e.indent+"- "+e.encodeKey(firstKey)+": "+firstVal)

				// Resto de propiedades indentadas
				for _, key := range keys[1:] {
					val := e.encodeValue(v[key], depth+1)
					lines = append(lines, indentation+e.indent+e.indent+e.encodeKey(key)+": "+val)
				}
			}

		case []interface{}:
			// Array en lista
			arrayStr := e.encodeArray(v, depth+1)
			if strings.Contains(arrayStr, "\n") {
				// Array multilínea - indentar cada línea
				arrayLines := strings.Split(arrayStr, "\n")
				for i, line := range arrayLines {
					if i == 0 {
						lines = append(lines, indentation+e.indent+"- "+line)
					} else {
						lines = append(lines, indentation+e.indent+"  "+line)
					}
				}
			} else {
				// Array inline
				lines = append(lines, indentation+e.indent+"- "+arrayStr)
			}

		default:
			// Primitivo en lista
			encoded := e.encodeValue(item, depth)
			lines = append(lines, indentation+e.indent+"- "+encoded)
		}
	}

	return strings.Join(lines, "\n")
}

func countTokensAPI(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")

	type request struct {
		Text string `json:"text"`
	}
	type response struct {
		Tokens               int `json:"tokens"`
		Words                int `json:"words"`
		Characters           int `json:"characters"`
		CharactersWithSpaces int `json:"charactersWithSpaces"`
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxPayloadSize)

	var req request
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		if err.Error() == "http: request body too large" {
			json.NewEncoder(w).Encode(response{})
			return
		}
		json.NewEncoder(w).Encode(response{})
		return
	}

	if len(req.Text) > 500000 {
		json.NewEncoder(w).Encode(response{})
		return
	}

	words := strings.Fields(req.Text)
	resp := response{
		Tokens:               countTokens(req.Text),
		Words:                len(words),
		Characters:           len(strings.ReplaceAll(req.Text, " ", "")),
		CharactersWithSpaces: len(req.Text),
	}

	json.NewEncoder(w).Encode(resp)
}

func countTokens(text string) int {
	initTokenizer()

	if tokenizerErr != nil {
		// Fallback a estimación si falla
		return countTokensEstimate(text)
	}

	tokens := tokenizer.Encode(text, nil, nil)
	return len(tokens)
}

// Mantener función de estimación como fallback
func countTokensEstimate(text string) int {
	text = strings.TrimSpace(text)
	if text == "" {
		return 0
	}

	re := regexp.MustCompile(`\w+|[^\w\s]`)
	basicTokens := re.FindAllString(text, -1)

	totalTokens := 0
	for _, token := range basicTokens {
		if len(token) > 8 && regexp.MustCompile(`^[a-zA-Z]+$`).MatchString(token) {
			if len(token) > 12 {
				totalTokens += 3
			} else {
				totalTokens += 2
			}
		} else {
			totalTokens += 1
		}
	}

	return totalTokens
}
