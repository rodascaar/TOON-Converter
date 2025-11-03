# TOON Converter

A modern, accessible Go-based web application that optimizes JSON for Large Language Models by converting it to TOON (Token-Oriented Object Notation) format, reducing token usage by 30-60%. Features a clean, responsive UI with tools for token counting, JSON repair, and TOON conversion.

## Features

- **TOON Converter**: Convert JSON to TOON format to reduce LLM token costs
- **Token Counter**: Count tokens in text for AI API cost estimation
- **JSON Repair Tool**: Automatically fix malformed JSON
- **Token Savings Calculator**: See exact savings when converting JSON to TOON
- **Modern UI**: Clean, accessible interface with keyboard navigation and screen reader support
- **Internationalization**: English/Spanish language support with localStorage persistence
- **Rate Limiting**: Built-in protection against abuse
- **Security Headers**: CORS, XSS, and other security measures
- **Performance Optimized**: Deferred script loading and efficient DOM manipulation
- **Docker Support**: Easy deployment with Docker and Docker Compose

## Quick Start

### Using Docker Compose (Recommended)

```bash
# Clone the repository
git clone <repository-url>
cd toon-converter

# Start the application
docker-compose up -d

# Access at http://localhost:8080
```

### Manual Build

```bash
# Install Go 1.24.1+
go mod download

# Build the application
go build -o main ./service

# Run the server
./main
```

## API Endpoints

### POST `/api/count-tokens`
Count tokens in text input.

**Request:**
```json
{
  "text": "Your text here"
}
```

**Response:**
```json
{
  "tokens": 42,
  "words": 8,
  "characters": 35,
  "charactersWithSpaces": 43
}
```

### POST `/api/fix-json`
Automatically repair malformed JSON.

**Request:**
```json
{
  "json": "{\"name\": \"John\", \"age\": 30,}"
}
```

**Response:**
```json
{
  "fixed": "{\"name\": \"John\", \"age\": 30}",
  "changes": ["Eliminada coma antes de }"]
}
```

### POST `/api/json-to-toon`
Convert JSON to TOON format with token savings calculation.

**Request:**
```json
{
  "json": "{\"name\": \"John\", \"age\": 30, \"city\": \"New York\"}",
  "delimiter": ",",
  "lengthMarker": false,
  "indent": 2
}
```

**Response:**
```json
{
  "toon": "name: John\nage: 30\ncity: New York",
  "tokenSavings": {
    "json": 15,
    "toon": 9,
    "saved": 6,
    "percentage": 40.0
  }
}
```

## TOON Format Specification

TOON (Token-Oriented Object Notation) is designed to minimize token usage in LLMs while maintaining readability:

### Objects
```json
{"name": "John", "age": 30}
```
```toon
name: John
age: 30
```

### Arrays
```json
["apple", "banana", "cherry"]
```
```toon
[3]: apple, banana, cherry
```

### Nested Objects
```json
{"user": {"name": "John", "age": 30}}
```
```toon
user:
  name: John
  age: 30
```

### Tabular Arrays
```json
[{"name": "John", "age": 30}, {"name": "Jane", "age": 25}]
```
```toon
[2]{name,age}:
    John,30
    Jane,25
```

## Development

### Prerequisites
- Go 1.24.1 or later
- Docker (optional)
- Modern web browser for frontend development

### Project Structure
```
toon-converter/
├── .github/
│   └── workflows/
│       └── ci.yml      # GitHub Actions CI/CD pipeline
├── service/           # Go backend
│   ├── main.go       # HTTP server and API endpoints
│   └── main_test.go  # Unit tests
├── static/           # Frontend assets
│   ├── index.html    # Main HTML page with SEO optimization
│   ├── style.css     # Modern CSS with accessibility features
│   ├── app.js        # Vanilla JavaScript with DOM manipulation
│   └── favicon_io/   # Favicon assets
├── docker-compose.yml # Docker Compose configuration
├── Dockerfile        # Docker build configuration
├── go.mod           # Go module dependencies
├── go.sum           # Go module checksums
└── README.md        # This file
```

### Running Tests
```bash
go test ./...
```

### Running Linting
```bash
go vet ./...
```

### Frontend Development
The frontend uses modern vanilla JavaScript with:
- DOM creation instead of innerHTML for security
- CSS classes instead of inline styles
- Keyboard accessibility and ARIA labels
- Feature detection instead of user-agent sniffing
- Deferred script loading for performance

### Building for Production
```bash
CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o main ./service
```

## Docker Deployment

### Build Image
```bash
docker build -t toon-converter .
```

### Run Container
```bash
docker run -p 8080:8080 toon-converter
```

## Configuration

The application uses the following default settings:
- **Port**: 8080
- **Rate Limit**: 5 requests/second per IP (burst: 10)
- **Max Payload**: 1MB per request
- **Timeout**: 5 seconds for TOON conversion, 10 seconds for HTTP

## Security Features

- Rate limiting per IP address
- Request size limits (1MB)
- CORS protection
- XSS prevention through DOM creation (no innerHTML)
- Content-Type validation
- HTML escaping for all dynamic content
- Automatic cleanup of old visitor data

## Performance

- Handles JSON up to 500KB per request
- TOON conversion timeout: 5 seconds
- Memory-efficient processing
- Concurrent request handling
- Deferred JavaScript loading
- Optimized DOM manipulation

## Accessibility

- WCAG 2.1 AA compliant
- Keyboard navigation support
- Screen reader compatibility with ARIA labels
- Focus management in modals
- High contrast support
- Semantic HTML structure

## Recent Improvements

### v1.0.1 - Code Quality & Accessibility Update

- **Security Enhancement**: Replaced `innerHTML` with DOM creation to prevent XSS attacks
- **Accessibility**: Added WCAG 2.1 AA compliance with keyboard navigation and ARIA labels
- **Performance**: Implemented deferred script loading and optimized DOM manipulation
- **Code Quality**: Extracted common logic, modernized browser detection, and improved maintainability
- **UI/UX**: Enhanced responsive design and user experience

### Key Technical Changes

- **Frontend Architecture**: Refactored JavaScript to use DOM APIs instead of string concatenation
- **Browser Compatibility**: Replaced user-agent sniffing with feature detection
- **CSS Organization**: Moved inline styles to reusable CSS classes
- **Error Handling**: Improved error messages and Safari-specific handling
- **Internationalization**: Maintained existing English/Spanish support

## Contributing

1. Fork the repository
2. Create a feature branch (`git checkout -b feature/amazing-feature`)
3. Make your changes following the established patterns:
   - Use DOM creation over innerHTML for security
   - Add ARIA labels for accessibility
   - Include CSS classes instead of inline styles
   - Test with keyboard navigation
4. Add tests if applicable
5. Submit a pull request

## License

MIT License - see LICENSE file for details.

## Related Projects

- [TOON Specification](https://github.com/johannschopplich/toon)
- [JSON Token Counter](https://github.com/openai/tiktoken)

## Support

For issues and questions:
- Open an issue on GitHub
- Check the FAQ in the web interface
- Review the API documentation above
- See recent improvements in the changelog above