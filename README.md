# 1. ImageApp — Setup & Run Guide

First, clone the repository:

```bash
git clone https://github.com/berndmehnert/imageapp.git
cd imageapp
```

## 1.1 Backend Setup

### 1.1.1 PostgreSQL + pgvector

**Option A: Docker (recommended)**

```bash
docker run -d \
  --name imagedb \
  -e POSTGRES_PASSWORD=postgres \
  -e POSTGRES_DB=imagedb \
  -p 5432:5432 \
  pgvector/pgvector:pg15
```

**Option B: Local Installation**

```bash
sudo apt install postgresql postgresql-16-pgvector
sudo -u postgres psql -c "CREATE DATABASE imagedb;"
sudo -u postgres psql -d imagedb -c "CREATE EXTENSION vector;"
```

### 1.1.2 ML Model Setup (one-time)

Create the model directory:

```bash
mkdir -p backend/model
```

We need an ONNX-compatible version of the sentence-transformers model `all-MiniLM-L6-v2`.

Install the Python dependencies (Python 3.10+):

```bash
pip install optimum[onnxruntime] transformers
```

Then, from the repository root, export the model:

```bash
python3 -c "
from optimum.onnxruntime import ORTModelForFeatureExtraction
from transformers import AutoTokenizer
model_name = 'sentence-transformers/all-MiniLM-L6-v2'
model = ORTModelForFeatureExtraction.from_pretrained(model_name, export=True)
tokenizer = AutoTokenizer.from_pretrained(model_name)
model.save_pretrained('./backend/model')
tokenizer.save_pretrained('./backend/model')
"
```

### 1.1.3 C Libraries

Download the ONNX Runtime and HuggingFace Tokenizer libraries and copy them into `backend/model/`:

**ONNX Runtime:**

```bash
wget https://github.com/microsoft/onnxruntime/releases/download/v1.24.1/onnxruntime-linux-x64-1.24.1.tgz
tar xzf onnxruntime-linux-x64-1.24.1.tgz
cp onnxruntime-linux-x64-1.24.1/lib/libonnxruntime.so ./backend/model/
```

**HuggingFace Tokenizer:**

```bash
wget https://github.com/daulet/tokenizers/releases/latest/download/libtokenizers.linux-amd64.tar.gz
tar xzf libtokenizers.linux-amd64.tar.gz
cp libtokenizers.a ./backend/model/
```

### 1.1.4 Verify Model Directory

Your `backend/model/` should now contain:

```
backend/model/
├── model.onnx           # ONNX model
├── tokenizer.json       # Tokenizer config
├── libonnxruntime.so    # ONNX Runtime library
└── libtokenizers.a      # Tokenizer library
```

### 1.1.5 Start the Backend

Make sure [Go 1.21+](https://go.dev/dl/) is installed, then from the `backend/` directory:

```bash
go mod tidy

CGO_LDFLAGS="-L./model" \
LD_LIBRARY_PATH=./model \
DATABASE_URL="postgres://postgres:postgres@localhost:5432/imagedb?sslmode=disable" \
go run ./cmd/server/main.go
```

You should see:

```
Server starting on :8080 
Seeding initial images...
Worker 0: processing complete for image 1
Worker 1: processing complete for image 2
Worker 2: processing complete for image 3
```

## 1.2 Frontend Setup

Make sure [Node.js 18+](https://nodejs.org/) is installed, then from the `frontend/` directory:

```bash
npm install
npm run dev
```

Open the provided link (usually `http://localhost:5173`) and enjoy! 🎉

## 1.3 Quick Reference

| Endpoint | Description |
|---|---|
| `POST /api/upload` | Upload image (multipart: image + title + tags) |
| `GET /api/feed` | Image feed with infinite scroll |
| `GET /api/feed?filter=cat` | Fuzzy filtered feed by tags |
| `WS /ws` | WebSocket for live updates |

## 1.4 Tech Stack

| Component | Technology |
|---|---|
| Frontend | React · Vite · TypeScript · Tailwind CSS |
| Backend | Go · Chi · gorilla/websocket |
| Database | PostgreSQL · pgvector |
| ML Model | all-MiniLM-L6-v2 (ONNX) |
| Imaging | disintegration/imaging |

# 2. Architecture, design decisions and todos
## 2.1 What is imageapp?
- imageapp is a full-stack image platform with semantic search capabilities.
## 2.2 How it works:
- User uploads an image with a title and tags via a multipart form
- A worker pool creates a 512×512 thumbnail and generates a 384-dimensional vector embedding from the tags using a sentence-transformer model (all-MiniLM-L6-v2) running as ONNX inference natively in Go
- The image, metadata, and embedding are stored in PostgreSQL with pgvector
- A WebSocket broadcast informs all connected frontends that new content is available
- The React frontend shows an infinite-scroll feed of thumbnails
- Users can fuzzy-filter by tags — the search query is embedded using the same 
   model and matched against stored embeddings via cosine similarity in pgvector
## 2.3 Design decisions
- The requirements of the test suggested semantic search
- Single database which can handle relational data and vector search was used, this was perfect for a test like this
## 2.4 Things that are missing and I had no time for
- Download of images
- Include title in filter
- Caching
- Tests
- Docker files
- Introduce users, authentication, authorization
# 3. Query examples by image
![Image 1](imageapp1.png)
![Image 2](imageapp2.png)
![Image 3](imageapp3.png)
![Image 4](imageapp4.png)

