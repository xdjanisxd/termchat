# TermChat

TermChat, özel odalarda çalışan Windows ve Linux uyumlu bir terminal sohbet uygulamasıdır. Bu repository'yi klonlayan kullanıcılar yalnızca istemciyi derleyerek mevcut TermChat sunucusuna bağlanabilir; istemci kullanımı için Docker veya PostgreSQL gerekmez.

## Gereksinimler

- [Git](https://git-scm.com/downloads)
- [Go 1.27 veya daha yeni](https://go.dev/dl/)
- İnternet bağlantısı ve modern bir terminal

Kurulumdan önce Go'yu doğrulayın:

```text
go version
```

## Windows

PowerShell açın ve sırasıyla çalıştırın:

```powershell
git clone https://github.com/xdjanisxd/termchat.git
cd termchat

New-Item -ItemType Directory -Force ./bin | Out-Null
go build -o ./bin/termchat.exe ./cmd/client

curl.exe -fsS "https://termchat.osmanela.com/healthz"
./bin/termchat.exe --server "https://termchat.osmanela.com"
```

Sağlık kontrolünün beklenen cevabı:

```json
{"status":"ok"}
```

Sunucu adresini geçerli PowerShell oturumu için kaydetmek isterseniz:

```powershell
$env:TERMCHAT_SERVER_URL = "https://termchat.osmanela.com"
./bin/termchat.exe
```

## Linux

Terminal açın ve sırasıyla çalıştırın:

```bash
git clone https://github.com/xdjanisxd/termchat.git
cd termchat

mkdir -p ./bin
CGO_ENABLED=0 go build -o ./bin/termchat ./cmd/client
chmod +x ./bin/termchat

curl -fsS "https://termchat.osmanela.com/healthz"
./bin/termchat --server "https://termchat.osmanela.com"
```

Sağlık kontrolünün beklenen cevabı:

```json
{"status":"ok"}
```

Sunucu adresini geçerli shell oturumu için kaydetmek isterseniz:

```bash
export TERMCHAT_SERVER_URL="https://termchat.osmanela.com"
./bin/termchat
```

## İlk kullanım

İstemci açıldığında yeni bir kullanıcı oluşturun veya mevcut hesabınızla giriş yapın. Temel komutlar:

| Komut | Açıklama |
|---|---|
| `/createroom <oda-adı> <parola>` | Özel oda oluşturur |
| `/join <oda-adı>` | Maskeli parola ekranıyla odaya katılır |
| `/who` | Odadaki çevrimiçi kullanıcıları gösterir |
| `/leave` | Odadan ayrılır |
| `/quit` | Uygulamayı kapatır |

Sunucu URL'sine `/v1` veya `/v1/ws` eklemeyin; istemci gerekli API ve WebSocket yollarını otomatik oluşturur.

## Sorun giderme

Sunucuya ulaşılamıyorsa önce şunu kontrol edin:

```text
https://termchat.osmanela.com/healthz
```

Eski veya geçersiz oturumu temizlemek için:

**Windows PowerShell**

```powershell
Remove-Item "$env:APPDATA\termchat\session.json" -Force -ErrorAction SilentlyContinue
```

**Linux**

```bash
rm -f "${XDG_CONFIG_HOME:-$HOME/.config}/termchat/session.json"
```

Sunucu tarafını kendi ortamınızda çalıştırmak için [Docker deployment rehberine](deploy/README.md) bakın.
