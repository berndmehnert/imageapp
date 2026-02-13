# How to run the imageapp
First clone the repository and go to the repository root:
```$ git clone https://github.com/berndmehnert/imageapp.git```
 
## 1. Run the server 
1. Install postgres 15+ and the pgvector extension and create a database, f.e. using docker (recommended)

docker run -d --name imagedb 
    -e POSTGRES_PASSWORD=postgres 
    -e POSTGRES_DB=imagedb 
    -p 5432:5432   pgvector/pgvector:pg15

or using your package manager, f.e.:

sudo apt install postgresql postgresql-16-pgvector
   sudo -u postgres psql -c "CREATE DATABASE imagedb;"
   sudo -u postgres psql -d imagedb -c "CREATE EXTENSION vector;"

2. In the `backend` folder of the cloned repo, please create a folder named `model`.
3. We need two C-libraries: Get the latest ONNX runtime and the HuggingFace Tokenizer and copy them into `backend/model`, f.e.:

wget https://github.com/microsoft/onnxruntime/releases/latest/download/onnxruntime-linux-x64-1.21.1.tgz
tar xzf onnxruntime-linux-x64-1.21.1.tgz
cp onnxruntime-linux-x64-1.21.1/lib/libonnxruntime.so ./backend/models/

and 

wget https://github.com/daulet/tokenizers/releases/latest/download/libtokenizers.linux-amd64.tar.gz
tar xzf libtokenizers.linux-amd64.tar.gz
cp libtokenizers.a ./backend/models/

4. We need an ONNX-compatible version of the sentence-transformers model "all-MiniLM-L6-v2". For this we use the following Python code (Python 3.10+). We need the result of the following code:


