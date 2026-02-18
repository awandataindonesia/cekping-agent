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
PINGVE_SERVER=api.cekping.id:50051
PINGVE_TOKEN=paste_token_disini
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
Jika menggunakan Docker, pastikan mode `--network host` aktif:

```bash
docker run -d \
  --name pingve-agent \
  --network host \
  --restart always \
  --env PINGVE_SERVER=api.cekping.id:50051 \
  --env PINGVE_TOKEN=paste_token_disini \
  ghcr.io/awandataindonesia/cekping-agent:latest
```

## Troubleshooting
- **Permission Denied**: Pastikan menjalankan dengan `sudo` (untuk Raw Socket).
- **Authentication Failed**: Cek kembali `AGENT_TOKEN` di `.env`.
