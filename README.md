# PingVe Agent

Agent lightweight untuk melakukan network monitoring (Ping & MTR).

## Persyaratan Sistem
- **OS**: Linux (Server/Desktop) atau macOS.
- **Network**: Koneksi internet stabil.
- **Privileges**: Akses **Root / Sudo** wajib diperlukan karena agent menggunakan *Raw Socket* untuk ICMP Ping yang akurat.

## Langkah Instalasi

### 1. Dapatkan Token
1. Register di web dashboard PingVe.
2. Masuk ke manu **Volunteer**.
3. Klik "Register New Probe" untuk mendapatkan `AGENT_TOKEN`.

### 2. Konfigurasi
Buat file `.env` di sebelah binary agent:

```env
SERVER_ADDR=api.cekping.com:50051
AGENT_TOKEN=paste_token_disini
```

### 3. Jalankan Agent

#### A. Menggunakan Binary (Disarankan)
Download binary sesuai OS anda, lalu jalankan dengan sudo:

```bash
# Contoh untuk Linux AMD64
chmod +x pingve-agent-linux-amd64
sudo ./pingve-agent-linux-amd64
```

#### B. Menggunakan Docker
Jika menggunakan Docker, pastikan mode `--privileged` aktif:

```bash
docker run -d \
  --name pingve-agent \
  --privileged \
  --restart always \
  --env-file .env \
  ghcr.io/awandataindonesia/pingve-agent:latest
```

## Troubleshooting
- **Permission Denied**: Pastikan menjalankan dengan `sudo` (untuk Raw Socket).
- **Authentication Failed**: Cek kembali `AGENT_TOKEN` di `.env`.
