# Terminal Chat Application

## 1. Proje Özeti

Terminal üzerinden çalışan, gerçek zamanlı ve çok kullanıcılı bir chat uygulaması geliştirmek istiyorum.

Uygulamada her kullanıcının kendine ait bir hesabı olacak. Kullanıcı terminal üzerinden uygulamayı başlatacak, hesabına giriş yapacak ve mevcut chat odalarını/lobilerini görebilecek.

Kullanıcı bir chat odasına katıldığında:

- Daha önce o odada gönderilmiş mesajları görebilecek.
- Odaya yeni mesaj geldikçe terminal ekranı gerçek zamanlı olarak güncellenecek.
- Kullanıcı terminal üzerinden mesaj gönderebilecek.
- Aynı odadaki diğer kullanıcılar mesajı anında görebilecek.
- Kullanıcı istediği zaman odadan ayrılabilecek.
- Daha sonra aynı odaya tekrar girdiğinde geçmiş mesajları tekrar görebilecek.

Temel fikir, Discord/IRC benzeri oda bazlı bir chat deneyimini tamamen terminal içerisinde sunmak.

---

# 2. Temel Kullanıcı Akışı

Uygulama terminalden çalıştırıldığında kullanıcı aşağıdaki akışı takip etmeli:

```text
Terminal
   ↓
Uygulamayı başlat
   ↓
Login / Register
   ↓
Lobby listesi
   ↓
Bir lobby seç
   ↓
Lobby'ye bağlan
   ↓
Eski mesajları yükle
   ↓
Real-time mesajlaşma
   ↓
Lobby'den çık
   ↓
Lobby listesine dön
```

Örneğin:

```bash
$ termchat
```

Ardından:

```text
Welcome to TermChat

1. Login
2. Register
3. Exit

>
```

Login sonrasında:

```text
Logged in as: burak

Available Rooms

1. general
2. programming
3. gaming
4. random

Select room:
>
```

Kullanıcı `programming` odasına girdiğinde:

```text
------------------------------------------------
# programming
------------------------------------------------

[17:32] alice: Has anyone tried Rust?
[17:33] bob: Yes, ownership takes some time.
[17:34] alice: I'm learning it now.

--- You joined the room ---

[17:35] burak: Hello everyone
[17:35] alice: Hey burak!
```

Yeni mesajlar geldikçe ekran otomatik olarak güncellenmeli.

---

# 3. Kullanıcı Sistemi

Her kullanıcının kendi hesabı olmalı.

Minimum kullanıcı modeli:

```text
User

id
username
password_hash
created_at
last_seen
```

Username unique olmalı.

Şifreler hiçbir zaman plain-text olarak saklanmamalı.

Password hashing için güvenli bir yöntem kullanılmalı:

```text
Argon2
```

veya

```text
bcrypt
```

Kullanıcı işlemleri:

```text
register
login
logout
```

İleride eklenebilecek özellikler:

```text
profile
display_name
avatar
bio
online_status
```

Ancak bunlar MVP kapsamında zorunlu değil.

---

# 4. Chat Rooms / Lobby Sistemi

Chat sistemi oda bazlı çalışmalı.

Örnek odalar:

```text
general
programming
gaming
music
random
```

Room modeli yaklaşık olarak:

```text
Room

id
name
description
created_at
created_by
```

MVP'de odalar sistem tarafından önceden oluşturulabilir.

İleride kullanıcıların kendi odalarını oluşturabilmesi desteklenebilir.

Örneğin:

```text
/create rust-developers
```

Ancak ilk sürüm için zorunlu değil.

---

# 5. Mesaj Sistemi

Her mesaj persistent olmalı.

Yani server restart edilse bile mesajlar kaybolmamalı.

Message modeli:

```text
Message

id
room_id
user_id
content
created_at
```

Kullanıcı bir odaya girdiğinde son mesajlar database üzerinden yüklenmeli.

Örneğin ilk etapta:

```text
last 50 messages
```

gösterilebilir.

Daha eski mesajlara erişmek ileride pagination ile yapılabilir.

---

# 6. Real-Time Communication

Chat gerçek zamanlı çalışmalı.

Kullanıcı bir mesaj gönderdiğinde aynı room içerisindeki bütün bağlı kullanıcılar mesajı anında almalı.

Tercih edilen iletişim modeli:

```text
WebSocket
```

Örnek bağlantı:

```text
Client A ─────┐
              │
Client B ─────┼── WebSocket Server
              │
Client C ─────┘
```

Server room bazlı connection yönetmeli.

Örneğin:

```text
room:general

alice
bob
charlie
```

Alice mesaj gönderdiğinde:

```text
alice
   ↓
server
   ↓
general room subscribers
   ↓
bob
charlie
```

---

# 7. Server Architecture

Uygulama client-server mimarisinde tasarlanmalı.

Basitleştirilmiş mimari:

```text
            ┌───────────────────┐
            │     Chat Server   │
            │                   │
            │ Auth              │
            │ Room Management   │
            │ WebSocket         │
            │ Message Service   │
            └─────────┬─────────┘
                      │
                ┌─────▼─────┐
                │ Database  │
                └───────────┘

        ▲              ▲              ▲
        │              │              │
   Terminal A     Terminal B     Terminal C
```

Server'ın sorumlulukları:

```text
authentication
user management
room management
message persistence
websocket connections
message broadcasting
authorization
```

---

# 8. Terminal Client

Terminal client mümkün olduğunca kullanıcı dostu olmalı.

Basit CLI yerine mümkünse TUI (Terminal User Interface) yaklaşımı kullanılabilir.

Örnek layout:

```text
┌──────────────────────────────────────────┐
│ # programming                           │
├──────────────────────────────────────────┤
│                                          │
│ alice: Anyone using Go?                  │
│ bob: Yes, for backend services.          │
│ burak: Same here.                        │
│                                          │
│ alice: Nice!                             │
│                                          │
├──────────────────────────────────────────┤
│ > Type message...                        │
└──────────────────────────────────────────┘
```

Terminal ekranında iki ana bölüm olabilir:

```text
message history
input field
```

Yeni mesaj geldiğinde message history otomatik güncellenmeli.

Input alanında kullanıcının yazdığı mesaj bozulmamalı.

---

# 9. Terminal Commands

Chat içerisinde bazı slash command'ler desteklenebilir.

MVP için:

```text
/help
/rooms
/leave
/quit
/who
```

Davranışlar:

```text
/help
```

Mevcut command'leri gösterir.

```text
/rooms
```

Room listesini gösterir.

```text
/leave
```

Mevcut room'dan çıkar.

```text
/quit
```

Uygulamayı kapatır.

```text
/who
```

Mevcut odadaki online kullanıcıları gösterir.

---

# 10. Mesaj Formatı

Mesajlar aşağıdaki formatta gösterilebilir:

```text
[17:42] username: message
```

Örneğin:

```text
[17:42] alice: hello
[17:43] bob: hey
[17:43] burak: what's up?
```

Server event formatı mümkünse structured olmalı.

Örneğin JSON:

```json
{
  "type": "message",
  "room_id": "programming",
  "user": {
    "id": "123",
    "username": "alice"
  },
  "content": "Hello everyone",
  "timestamp": "2026-08-26T17:42:00Z"
}
```

---

# 11. WebSocket Eventleri

Client ve server arasındaki protocol mümkün olduğunca açık tanımlanmalı.

Örnek client eventleri:

```text
join_room
leave_room
send_message
typing
ping
```

Server eventleri:

```text
room_joined
room_left
new_message
user_joined
user_left
error
pong
```

MVP için minimum gerekli eventler:

```text
join_room
leave_room
send_message
new_message
```

---

# 12. Authentication

Login sonrasında server kullanıcıya bir session/token vermeli.

Örneğin:

```text
JWT
```

veya server-side session kullanılabilir.

Client token'ı local olarak saklayabilir fakat güvenli bir yöntem tercih edilmeli.

WebSocket bağlantısı açılırken authentication kontrol edilmeli.

Örneğin:

```text
terminal client

     ↓ login

POST /auth/login

     ↓

access token

     ↓

WebSocket connection

     ↓

authenticated chat connection
```

---

# 13. REST API

Authentication ve temel işlemler HTTP API üzerinden yapılabilir.

Örnek endpoint'ler:

```text
POST /auth/register
POST /auth/login

GET /rooms
GET /rooms/:id/messages

GET /users/me
```

Chat mesajları ise WebSocket üzerinden ilerlemeli.

---

# 14. Database

İlk sürüm için:

```text
PostgreSQL
```

tercih edilebilir.

Development aşamasında SQLite kullanılabilir ancak production architecture PostgreSQL'e uygun tasarlanmalı.

Temel tablolar:

```text
users
rooms
messages
```

İlişki:

```text
users
  │
  │
  └──── messages
          │
          │
rooms ────┘
```

---

# 15. Önerilen Teknik Stack

Agent uygun gördüğü alternatifleri önerebilir ancak başlangıç için aşağıdaki yapı tercih edilebilir.

## Backend

Seçeneklerden biri:

```text
Go
```

Avantajları:

```text
concurrency
WebSocket handling
tek binary deployment
yüksek performans
```

Önerilen yapı:

```text
Go
PostgreSQL
WebSocket
REST API
JWT
```

Alternatif olarak:

```text
Node.js + TypeScript
```

da kullanılabilir.

---

## Terminal Client

Eğer Go kullanılıyorsa:

```text
Bubble Tea
Lip Gloss
```

gibi TUI library'leri kullanılabilir.

Client tek executable olabilir:

```bash
termchat
```

---

# 16. Önerilen Repository Yapısı

Monorepo yaklaşımı kullanılabilir.

```text
termchat/

├── server/
│   ├── cmd/
│   ├── internal/
│   │   ├── auth/
│   │   ├── users/
│   │   ├── rooms/
│   │   ├── messages/
│   │   └── websocket/
│   │
│   └── main.go
│
├── client/
│   ├── cmd/
│   ├── internal/
│   │   ├── api/
│   │   ├── websocket/
│   │   └── ui/
│   │
│   └── main.go
│
├── migrations/
│
├── docker-compose.yml
│
├── README.md
│
└── idea.md
```

Bu yapı sadece öneridir. Agent daha mantıklı bir architecture görürse değiştirebilir.

---

# 17. MVP Scope

İlk sürüm mümkün olduğunca küçük tutulmalı.

MVP içerisinde kesinlikle bulunması gerekenler:

```text
user registration
user login

room listing

join room
leave room

message history

send message

real-time receiving messages

message persistence

terminal UI
```

MVP dışında bırakılabilecek özellikler:

```text
private messages
friend system
message reactions
message editing
message deletion
file sharing
images
voice chat
notifications
custom rooms
moderation
admin dashboard
end-to-end encryption
```

Öncelik temel chat deneyimini stabil hale getirmek.

---

# 18. Kritik Teknik Gereksinimler

Uygulamada aşağıdaki durumlara özellikle dikkat edilmeli.

### Connection Handling

WebSocket bağlantısı koparsa client mümkünse otomatik reconnect deneyebilmeli.

Örneğin:

```text
connection lost

reconnecting...

connected
```

---

### Duplicate Messages

Reconnect sonrasında aynı mesajın iki kere gösterilmesi engellenmeli.

Mesajların unique ID'si olmalı.

---

### Message Ordering

Mesajlar doğru sırada gösterilmeli.

Server timestamp authoritative olmalı.

---

### Concurrent Users

Aynı room içerisinde birden fazla kullanıcı aynı anda mesaj gönderebilmeli.

Race condition oluşmamalı.

---

### Input Handling

Kullanıcı mesaj yazarken yeni mesaj gelmesi terminal input alanını bozmamalı.

TUI framework bu problemi düzgün yönetmeli.

---

### Security

Minimum olarak:

```text
password hashing
input validation
authentication
authorization
rate limiting
message length limit
```

olmalı.

---

# 19. Message Limits

İlk sürüm için bazı limitler konulabilir.

Örneğin:

```text
username:
3-24 characters

message:
max 2000 characters

room name:
3-32 characters
```

Rate limiting örneği:

```text
max 5 messages / second / user
```

Bu değerler daha sonra değiştirilebilir.

---

# 20. Future Features

MVP tamamlandıktan sonra aşağıdaki özellikler değerlendirilebilir:

```text
private messages

user mentions
@username

message reactions

message editing

message deletion

custom rooms

private rooms

room passwords

moderators

admins

ban / mute system

message search

typing indicators

read receipts

online / away status

terminal themes

config file

desktop notifications

encrypted private messages

file sharing
```

---

# 21. CLI Kullanımı

İdeal olarak uygulama kurulunca:

```bash
termchat
```

ile açılmalı.

İleride direkt room'a bağlanma da desteklenebilir:

```bash
termchat join programming
```

veya:

```bash
termchat --room programming
```

Server adresi config üzerinden değiştirilebilir:

```bash
termchat --server chat.example.com
```

Config:

```text
~/.config/termchat/config.toml
```

örneğin:

```toml
server = "https://chat.example.com"
```

---

# 22. Development Environment

Local development mümkün olduğunca kolay olmalı.

Tercihen:

```bash
docker compose up
```

ile database ve server ayağa kalkabilmeli.

Örneğin:

```text
PostgreSQL
Chat Server
```

Docker Compose üzerinden çalıştırılabilir.

Client local terminalden çalıştırılabilir.

---

# 23. Agent İçin Görev

Bu dökümanı okuyarak projeyi planla ve geliştir.

Öncelikle gereksiz over-engineering yapmadan temiz bir MVP oluştur.

Başlamadan önce:

1. Architecture'ı belirle.
2. Kullanılacak teknoloji stack'ini belirle.
3. Database schema'yı tasarla.
4. Client-server protocol'ünü tanımla.
5. WebSocket eventlerini belirle.
6. Repository yapısını oluştur.
7. Ardından implementasyona başla.

Kod içerisinde:

- temiz ve anlaşılır naming kullan,
- modüler architecture oluştur,
- gereksiz abstraction yapma,
- error handling'i ihmal etme,
- concurrency problemlerine dikkat et,
- güvenlik açısından temel önlemleri uygula.

Öncelik sırası:

```text
1. çalışan server
2. authentication
3. database
4. rooms
5. WebSocket chat
6. message persistence
7. terminal client
8. reconnect/error handling
9. polish
```

Her aşamada uygulamanın çalışabilir durumda kalmasını hedefle.

Amaç production-scale Discord alternatifi yapmak değil.

Amaç:

> Terminal üzerinden kullanılabilen, hesap sistemi olan, oda bazlı, mesaj geçmişi saklayan ve gerçek zamanlı çalışan temiz bir chat uygulamasının sağlam MVP'sini oluşturmak.
