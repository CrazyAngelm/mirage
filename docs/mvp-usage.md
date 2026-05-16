# MIRAGE — Краткая инструкция

## Сервер (SERVER_IP)

**Сейчас:** запущен и работает.

Проверить:
```bash
ssh ubuntu@SERVER_IP
sudo systemctl status mirage-server --no-pager
sudo /opt/mirage/mirage-server doctor
```

Перезапустить:
```bash
sudo systemctl restart mirage-server
```

Получить клиентскую ссылку:
```bash
sudo /opt/mirage/mirage-server show-link
```

**Файлы:**
```
/opt/mirage/
  mirage-server          # бинарник
  bin/xray               # TCP/443
  bin/hysteria           # UDP/443
  configs/mirage.yaml
  configs/xray.json
  configs/hysteria.yaml
  state/default.link     # клиентская ссылка
```

**Порты:** 443/tcp, 443/udp, 51820/udp

---

## Windows клиент (этот ПК)

**Сейчас:** sing-box запущен (pid 3028), прокси на `127.0.0.1:2080`.

Проверить:
```powershell
curl.exe -x http://127.0.0.1:2080 https://ifconfig.me/ip
# Должно вернуть: SERVER_IP
```

---

## Режимы работы

| Режим | Что делает | Требования |
|-------|-----------|------------|
| `normal` | Mixed proxy на `127.0.0.1:2080`, все транспорты | — |
| `cheap` | Mixed proxy, один лучший транспорт | — |
| `full` | TUN VPN, системный туннель | **Administrator** |

---

## GUI режим (рекомендуется)

Дважды кликни `mirage-client.exe` — появится иконка в трее (рядом с часами).

**Tray меню:**
- **Connect** — запускает VPN
- **Disconnect** — останавливает VPN
- **Mode** → Normal / Cheap / Full (TUN)
- **Import Link...** — импорт профиля из файла
- **Show Status** — текущий статус
- **Exit** — выход

**System proxy:** В режимах `normal` и `cheap` системный прокси автоматически включается при Connect и отключается при Disconnect. Браузеры (Chrome, Edge) подхватывают автоматически.

**TUN режим:** Требует запуск от Administrator. Включает системный VPN-туннель — весь трафик идёт через него, прокси вручную настраивать не нужно.

---

## CLI режим

### Импорт профиля

```powershell
cd D:\Projects\VPN
.\mirage-client.exe import prod-link.txt
```

### Выбор режима

```powershell
.\mirage-client.exe mode normal   # mixed proxy, все транспорты
.\mirage-client.exe mode cheap    # mixed proxy, один транспорт
.\mirage-client.exe mode full     # TUN VPN (требует admin)
```

### Запуск

```powershell
# Запуск в foreground (блокирует терминал)
.\mirage-client.exe connect

# Для TUN режима — от имени Administrator:
Start-Process .\mirage-client.exe -ArgumentList "connect" -Verb runAs
```

При запуске `connect` в режимах `normal`/`cheap` системный прокси автоматически включается.

### Остановка

```powershell
.\mirage-client.exe disconnect
```

Убивает sing-box и сбрасывает системный прокси.

### Диагностика

```powershell
.\mirage-client.exe doctor
```

---

## Файлы клиента

```
%ProgramData%\Mirage\
  bin\sing-box.exe
  configs\sing-box.json      # конфиг sing-box (mixed или TUN)
  configs\xray-client.json   # конфиг xray (опционально)
  state\profile.link         # импортированная ссылка
  state\mode.txt             # текущий режим (normal/cheap/full)
  logs\sing-box.log          # логи sing-box
```

---

## Генерация Xray client config (опционально)

```powershell
cd D:\Projects\VPN
.\mirage-client.exe prepare-xray
```

Конфиг появится в:
```
%ProgramData%\Mirage\configs\xray-client.json
```

Для запуска xray на Windows скачай `Xray-windows-64.zip` с GitHub XTLS/Xray-core, положи `xray.exe` в `%ProgramData%\Mirage\bin\`, потом:
```powershell
& "$env:ProgramData\Mirage\bin\xray.exe" run -config "$env:ProgramData\Mirage\configs\xray-client.json"
```

Xray поднимет SOCKS5 на `127.0.0.1:2081`.

---

## Устранение неполадок

**`curl` не возвращает SERVER_IP:**
1. Проверь, запущен ли sing-box: `Get-Process sing-box`
2. Проверь doctor: `.\mirage-client.exe doctor`
3. Проверь сервер: `ssh ... && sudo systemctl status mirage-server`

**Браузер не использует прокси:**
- Убедись, что режим `normal` или `cheap` (не `full`)
- Проверь `mirage-client.exe doctor` — должно быть `local-2080: OK`
- В Windows Settings → Proxy должен стоять `127.0.0.1:2080`

**TUN режим не работает:**
- Запусти `mirage-client.exe` от имени Administrator
- Проверь `doctor` — будет подсказка про admin права

**"sing-box missing":**
- Скачай с [GitHub SagerNet/sing-box releases](https://github.com/SagerNet/sing-box/releases)
- Положи `sing-box.exe` в `%ProgramData%\Mirage\bin\`
