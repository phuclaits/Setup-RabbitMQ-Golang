# Sử dụng Golang làm base image
FROM golang:1.23

# Thiết lập thư mục làm việc bên trong container
WORKDIR /app

# Sao chép các file module và go.sum trước
COPY go.mod go.sum ./

# Tải các module cần thiết
RUN go mod download

# Sao chép mã nguồn của ứng dụng vào container
COPY app/ ./app

# Di chuyển vào thư mục ứng dụng
WORKDIR /app/app

# Biên dịch mã nguồn thành file thực thi
RUN go build -o main .

# Lệnh khởi chạy ứng dụng
CMD ["./main"]
