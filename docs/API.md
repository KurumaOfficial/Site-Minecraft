# API ESTELAR.SU — простыми словами

> Эта документация написана так, чтобы её мог прочитать **любой** —
> владелец сервера, плагин-разработчик и даже школьник. Никакого
> жаргона, только пошаговые примеры и `curl`-команды, которые можно
> копировать целиком.

## Содержание

1. [Что вообще такое этот API](#что-вообще-такое-этот-api)
2. [Базовый адрес и формат данных](#базовый-адрес-и-формат-данных)
3. [Авторизация: где нужна, где нет](#авторизация-где-нужна-где-нет)
4. [Глоссарий за 60 секунд](#глоссарий-за-60-секунд)
5. [Эндпоинты для покупателя (публичные)](#эндпоинты-для-покупателя-публичные)
   * [`GET /api/v1/health`](#-get-apiv1health--проверка-что-сервер-жив)
   * [`GET /api/v1/meta`](#-get-apiv1meta--бренд-и-настройки)
   * [`GET /api/v1/catalog`](#-get-apiv1catalog--список-товаров)
   * [`POST /api/v1/orders/quote`](#-post-apiv1ordersquote--рассчитать-стоимость-с-промокодом)
   * [`POST /api/v1/orders`](#-post-apiv1orders--оформить-заказ)
   * [`GET /api/v1/orders/:id`](#-get-apiv1ordersid--узнать-статус-заказа)
   * [`POST /api/v1/orders/:id/payment`](#-post-apiv1ordersidpayment--инициировать-оплату)
   * [`POST /api/v1/payments/yookassa/webhook`](#-post-apiv1paymentsyookassawebhook--приём-уведомления-юkassa)
6. [Эндпоинты админки (для владельца сайта)](#эндпоинты-админки-для-владельца-сайта)
   * [`GET /api/v1/admin/dashboard`](#-get-apiv1admindashboard--дашборд)
   * [`GET /api/v1/admin/orders` / `PATCH /api/v1/admin/orders/:id`](#-get-apiv1adminorders--patch-apiv1adminordersid--заявки)
   * [`GET /api/v1/admin/catalog` / `POST /api/v1/admin/catalog`](#-get-apiv1admincatalog--post-apiv1admincatalog--каталог)
   * [`GET /api/v1/admin/promos` / `POST /api/v1/admin/promos`](#-get-apiv1adminpromos--post-apiv1adminpromos--промокоды)
   * [`GET /api/v1/admin/integrations` / `POST /api/v1/admin/integrations`](#-get-apiv1adminintegrations--post-apiv1adminintegrations--настройки-плагина)
   * [`POST /api/v1/admin/integrations/test`](#-post-apiv1adminintegrationstest--тестовый-webhook)
   * [`POST /api/v1/admin/payments`](#-post-apiv1adminpayments--настройки-оплаты)
7. [Webhook'и в плагин Minecraft](#webhookи-в-плагин-minecraft)
8. [Ошибки: как они выглядят](#ошибки-как-они-выглядят)
9. [Лимиты и идемпотентность](#лимиты-и-идемпотентность)

---

## Что вообще такое этот API

Это «дверь», через которую к серверу обращаются:

* **сайт-витрина** (страница покупателя) — тянет каталог, делает заказ;
* **админка** в браузере владельца — управляет товарами и заявками;
* **плагин Minecraft** — слушает webhook'и про оплаченные заказы и
  выдаёт игрокам привилегии/кейсы/валюту автоматически;
* **платёжная система ЮKassa** — присылает уведомление об успешной оплате.

Все они говорят с одним сервером по HTTP. Этот документ описывает,
**какие URL дёрнуть и что прислать в запросе**, чтобы получить нужный
ответ.

## Базовый адрес и формат данных

* **База:** `https://estelar.su` в продакшене или `http://localhost:8080`
  локально (порт можно переопределить через переменную `APP_PORT`).
* **Все эндпоинты живут под префиксом `/api/v1`.**
* **Формат запроса/ответа — JSON в UTF-8.** Не XML, не form-data, не
  protobuf. Только JSON.
* В каждом запросе с телом обязательно отправляйте заголовок
  `Content-Type: application/json`.

## Авторизация: где нужна, где нет

| Тип эндпоинта | Кто им пользуется | Авторизация |
|---|---|---|
| `/api/v1/health`, `/api/v1/meta`, `/api/v1/catalog` | Любой посетитель | ❌ нет |
| `/api/v1/orders/...` | Покупатель | ❌ нет (rate-limit по IP) |
| `/api/v1/payments/yookassa/webhook` | Серверы ЮKassa | ❌ (внутри backend сам перепроверяет платёж по API ЮKassa) |
| `/api/v1/admin/...` | Владелец сайта | ✅ JWT-токен Supabase Auth (Discord login) |

Если эндпоинт требует авторизацию, в каждом запросе нужно прислать заголовок:

```
Authorization: Bearer <jwt_token>
```

Где взять `<jwt_token>`: войти в админку в браузере через Discord, и
скопировать `access_token` из объекта `supabase.auth.session()`. Либо
использовать тот же токен, что хранится в `localStorage`. В обычной
работе токен подставляет JS админки сам.

## Глоссарий за 60 секунд

* **Заказ (order)** — одна попытка покупки. Имеет уникальный
  человекочитаемый ID вида `ESTELAR-20260426-AB12CD34`.
* **Категория** — тип товара: `privilege` (привилегия), `case` (кейс),
  `currency` (донат-валюта).
* **Период** — срок действия привилегии (например, `forever`, `30d`).
* **Промокод** — короткий код, дающий скидку в процентах от цены.
* **Provider (платёжный провайдер)** — `manual` (заявка идёт в ручную
  очередь админа) или `yookassa` (автоматическая оплата через ЮKassa).
* **Webhook (вебхук)** — HTTP-запрос, который backend сам инициирует,
  чтобы рассказать плагину Minecraft «вот этому игроку выдай вот это».

---

# Эндпоинты для покупателя (публичные)

## ◆ `GET /api/v1/health` — проверка, что сервер жив

```bash
curl -s https://estelar.su/api/v1/health
```

```json
{
  "status": "ok",
  "timestamp": "2026-04-26T20:15:33Z",
  "projectName": "ESTELAR.SU",
  "serverName": "ESTELAR",
  "storageMode": "supabase",
  "databaseProvider": "supabase"
}
```

**Когда нужен:** мониторинг (Uptime Robot и т.п.), CI-проверка после деплоя.

---

## ◆ `GET /api/v1/meta` — бренд и настройки

Возвращает название проекта, ссылки на оферту/политику, режим
ручной/автоматической выдачи. Используется фронтом, чтобы не хардкодить
эти строки в HTML.

```bash
curl -s https://estelar.su/api/v1/meta
```

---

## ◆ `GET /api/v1/catalog` — список товаров

Главный эндпоинт витрины: возвращает массив товаров с описаниями,
картинками, периодами и ценами.

```bash
curl -s https://estelar.su/api/v1/catalog
```

```json
{
  "items": [
    {
      "slug": "vip",
      "name": "VIP",
      "category": "privilege",
      "categoryLabel": "Привилегия",
      "summary": "Цветной ник, /home x3, /fly",
      "image": "/assets/img/vip.png",
      "currency": "RUB",
      "periods": [
        {"code": "30d",     "label": "30 дней",  "price": 199, "isDefault": true},
        {"code": "forever", "label": "Навсегда", "price": 999}
      ],
      "highlights": ["Цветной ник", "Дополнительные дома", "Полёт"]
    }
  ]
}
```

**Что важно:** возвращаются только активные товары (`is_active = true`).
Никаких приватных полей вроде `external_id` тут нет.

---

## ◆ `POST /api/v1/orders/quote` — рассчитать стоимость с промокодом

Вызывайте перед оформлением, чтобы показать покупателю «итог» с
учётом промокода. Никакого заказа в базе не создаётся.

```bash
curl -s https://estelar.su/api/v1/orders/quote \
  -H 'Content-Type: application/json' \
  -d '{"productSlug":"vip","periodCode":"30d","promoCode":"SUMMER10","quantity":1}'
```

```json
{
  "productSlug": "vip",
  "productName": "VIP",
  "periodCode": "30d",
  "periodLabel": "30 дней",
  "quantity": 1,
  "unitPrice": 199,
  "basePrice": 199,
  "discountAmount": 19,
  "finalPrice": 180,
  "currency": "RUB",
  "promo": {
    "code": "SUMMER10",
    "applied": true,
    "percent": 10,
    "message": "Промокод применен."
  }
}
```

Если промокод неверный — `applied: false` и `message` поясняет почему;
заказ всё равно можно создать (просто без скидки).

---

## ◆ `POST /api/v1/orders` — оформить заказ

Создаёт заявку. После этого её нужно либо оплатить (см. следующий
эндпоинт), либо ждать ручной обработки (если `provider = manual`).

```bash
curl -s https://estelar.su/api/v1/orders \
  -H 'Content-Type: application/json' \
  -d '{
    "productSlug": "vip",
    "periodCode": "30d",
    "nickname": "Steve",
    "promoCode": "SUMMER10",
    "quantity": 1
  }'
```

```json
{
  "order": {
    "id": "ESTELAR-20260426-AB12CD34",
    "status": "pending",
    "statusLabel": "Ожидает ручной выдачи",
    "finalPrice": 180,
    "currency": "RUB",
    "nickname": "Steve",
    "...": "..."
  }
}
```

**Важные правила:**

* `nickname` — латиница/цифры/`_`, длина 3–16 символов (стандарт Mojang);
* `quantity` — для штучных товаров (например, кейсы) можно указать
  больше 1, для привилегий и единичных товаров игнорируется;
* rate-limit: ~10 заказов в минуту с одного IP, превышение → 429.

---

## ◆ `GET /api/v1/orders/:id` — узнать статус заказа

```bash
curl -s https://estelar.su/api/v1/orders/ESTELAR-20260426-AB12CD34
```

Возвращает текущее состояние заказа (`pending` / `review` / `issued` /
`rejected`). Используйте для страницы «спасибо за покупку, мы выдадим
ваш товар, как только проверим оплату».

---

## ◆ `POST /api/v1/orders/:id/payment` — инициировать оплату

После создания заказа фронт сразу вызывает этот эндпоинт. Backend
смотрит на текущего провайдера в настройках админки и:

* **`provider = manual`** → возвращает `{manual: true, message: "..."}`.
  Никакого редиректа — фронт показывает «спасибо, заявка принята,
  админ выдаст в ближайшее время».
* **`provider = yookassa`** → создаёт платеж в API ЮKassa и возвращает
  `redirectUrl` на форму оплаты. Фронт делает `window.location = redirectUrl`.

```bash
curl -s -X POST https://estelar.su/api/v1/orders/ESTELAR-20260426-AB12CD34/payment
```

```json
{
  "provider": "yookassa",
  "orderId": "ESTELAR-20260426-AB12CD34",
  "paymentId": "2c8a7b1b-000f-5000-8000-1234567890ab",
  "redirectUrl": "https://yoomoney.ru/checkout/payments/v2/...",
  "manual": false,
  "message": "Оплата ЮKassa: пользователя нужно перенаправить на redirectUrl."
}
```

---

## ◆ `POST /api/v1/payments/yookassa/webhook` — приём уведомления ЮKassa

Этот URL вы прописываете в личном кабинете ЮKassa: «Уведомления →
HTTP-уведомления → URL». Вызывает не покупатель, а сами серверы
ЮKassa.

**Что делает backend:**

1. Принимает тело уведомления с `object.id` (ID платежа).
2. **Перепроверяет** платеж: делает `GET /v3/payments/{id}` к API
   ЮKassa с Basic Auth (`shopId:secretKey`). Это значит, что даже если
   кто-то подделает уведомление и пришлёт его сам, backend увидит
   реальный статус «pending» и не выдаст товар.
3. Если статус = `succeeded` — переводит заказ в `issued` и автоматически
   шлёт webhook в плагин Minecraft (см. ниже).
4. Возвращает `200 OK` (всегда — иначе ЮKassa будет повторять до 24 часов).

---

# Эндпоинты админки (для владельца сайта)

> Все требуют `Authorization: Bearer <jwt>`. Без токена → `401`. С
> токеном, но без права (Discord ID не в whitelist) → `403`.

## ◆ `GET /api/v1/admin/dashboard` — дашборд

Возвращает агрегированную статистику: сегодняшние заказы, выручка за
30 дней, разбивка по статусам.

## ◆ `GET /api/v1/admin/orders` / `PATCH /api/v1/admin/orders/:id` — заявки

Список и изменение статуса:

```bash
curl -H "Authorization: Bearer $JWT" \
     -X PATCH https://estelar.su/api/v1/admin/orders/ESTELAR-20260426-AB12CD34 \
     -H 'Content-Type: application/json' \
     -d '{"status":"issued","adminNote":"Выдано в /priv give"}'
```

Допустимые статусы: `pending`, `review`, `issued`, `rejected`.

## ◆ `GET /api/v1/admin/catalog` / `POST /api/v1/admin/catalog` — каталог

`POST` принимает полный JSON каталога — это «снимок», заменяет всё
целиком (так проще: никаких diff'ов).

## ◆ `GET /api/v1/admin/promos` / `POST /api/v1/admin/promos` — промокоды

Добавление/обновление промокода. Поля: `code`, `discountPercent`,
`isActive`, `usageLimit?`, `startsAt?`, `endsAt?`.

## ◆ `GET /api/v1/admin/integrations` / `POST /api/v1/admin/integrations` — настройки плагина

Получает/сохраняет три webhook-эндпоинта (`privileges`, `cases`,
`currency`) и `webhookSecret` (HMAC-ключ, которым плагин проверяет
подлинность приходящих уведомлений).

```bash
curl -H "Authorization: Bearer $JWT" \
     -X POST https://estelar.su/api/v1/admin/integrations \
     -H 'Content-Type: application/json' \
     -d '{
       "privilegesEndpoint": {"enabled": true, "url": "https://mc.example/api/privileges", "token": "secret-token"},
       "casesEndpoint":      {"enabled": true, "url": "https://mc.example/api/cases",      "token": ""},
       "currencyEndpoint":   {"enabled": false, "url": "", "token": ""},
       "webhookSecret": "32-char-shared-secret"
     }'
```

При чтении (`GET`) секреты возвращаются как `••••••••` — на диске они
лежат в открытом виде, в API в открытом виде НЕ показываем.

## ◆ `POST /api/v1/admin/integrations/test` — тестовый webhook

Шлёт фейковый webhook на сконфигурированный URL. Полезно проверить,
что плагин действительно отвечает 200 OK.

```bash
curl -H "Authorization: Bearer $JWT" \
     -X POST https://estelar.su/api/v1/admin/integrations/test \
     -H 'Content-Type: application/json' \
     -d '{"category":"privilege"}'
```

```json
{
  "result": {
    "ok": true,
    "statusCode": 200,
    "message": "Получен ответ 200 (OK)"
  }
}
```

## ◆ `POST /api/v1/admin/payments` — настройки оплаты

```json
{
  "provider": "yookassa",
  "testMode": true,
  "shopId": "1234567",
  "secretKey": "test_AbCDEFGhijKLMnopqrSTUv",
  "returnUrl": "https://estelar.su/order-success",
  "webhookUrl": "https://estelar.su/api/v1/payments/yookassa/webhook",
  "description": "ESTELAR.SU"
}
```

Поддерживаемые провайдеры: `manual`, `yookassa`. Любой другой → `400`.

---

# Webhook'и в плагин Minecraft

Когда заказ переходит в статус `issued` (вручную через админку или
автоматически после оплаты ЮKassa) — backend шлёт `POST` на сконфи-
гурированный URL соответствующей категории.

**Заголовки:**

```
Content-Type: application/json
User-Agent: estelar-backend/1.0
Authorization: Bearer <token>            # если задан в integrations
X-ESTELAR-Signature: sha256=<hex_hmac>   # если задан webhookSecret
```

**Тело:**

```json
{
  "event": "order.issued",
  "orderId": "ESTELAR-20260426-AB12CD34",
  "productSlug": "vip",
  "productName": "VIP",
  "category": "privilege",
  "categoryLabel": "Привилегия",
  "nickname": "Steve",
  "periodCode": "30d",
  "periodLabel": "30 дней",
  "quantity": 1,
  "finalPrice": 180,
  "currency": "RUB",
  "handledBy": "payment:yookassa",
  "createdAt": "2026-04-26T20:14:55Z",
  "sentAt":    "2026-04-26T20:15:09Z"
}
```

**Как плагину проверить подлинность:**

```python
import hmac, hashlib
expected = hmac.new(secret.encode(), body_bytes, hashlib.sha256).hexdigest()
assert request.headers["X-ESTELAR-Signature"] == "sha256=" + expected
```

Если подпись не совпала — НЕ выдавать товар.

---

# Ошибки: как они выглядят

```json
{
  "error": "Промокод просрочен.",
  "requestId": "01J3..."
}
```

* `400` — невалидные данные (опечатка в JSON, неверный nickname);
* `401` — нет токена / токен просрочен;
* `403` — токен есть, но Discord-аккаунт не в whitelist админов;
* `404` — заказа/товара нет;
* `409` — конфликт (например, повторное оформление того же заказа);
* `429` — превышен rate-limit;
* `500` — внутренняя ошибка (с `requestId`, который видно в логах backend'а).

---

# Лимиты и идемпотентность

* Глобальный rate-limit на API: **120 req/min** на IP.
* На `/api/v1/orders*`: **10 req/min** на IP.
* На `/api/v1/admin/*`: **300 req/min** на IP (для удобной работы в админке).
* Платёж в YooKassa создаётся с `Idempotence-Key`, поэтому повторный
  клик «оплатить» в течение секунды не приведёт к двойному списанию.
* Webhook на плагин — **best effort**: если плагин ответил 5xx, заказ
  всё равно остаётся в `issued`; админ видит это в журнале и может
  переотправить вручную.

---

# LuckPerms: как принять webhook на сервере (живой пример)

Самый частый сценарий — выдача привилегий через [LuckPerms](https://luckperms.net/). Ниже — минимальный Bukkit-плагин, который слушает webhook ESTELAR на `:25580`, проверяет HMAC-SHA256 и выполняет `lp user … parent addtemp …` от имени консоли. Этот же плагин у нас уже использован для практического теста.

## Шаг 1 — скачать LuckPerms

Положите [LuckPerms-Bukkit.jar](https://luckperms.net/download) в `plugins/` сервера Paper/Spigot.

## Шаг 2 — создать группы

После запуска сервера один раз выполните в консоли:

```
lp creategroup vip
lp creategroup premium
lp creategroup deluxe
```

(Вместо vip/premium/deluxe используйте `slug` ваших товаров из админки.)

## Шаг 3 — мост ESTELAR → LuckPerms

Простейший Bukkit-плагин на Java 17 (`EstelarBridge.jar`), исходник:

```java
public class EstelarBridge extends JavaPlugin {
  // ...
  void onWebhook(byte[] body, String sig) {
    // 1) проверяем HMAC-SHA256
    if (!verify(body, sig, getConfig().getString("secret"))) {
      respond(401, "{\"ok\":false,\"error\":\"bad signature\"}"); return;
    }
    // 2) парсим JSON
    String nickname = extract(body, "nickname"); // напр. "Kuruma"
    String slug     = extract(body, "productSlug"); // напр. "vip"
    String period   = extract(body, "periodLabel"); // "30 дней", "Навсегда"
    String duration = mapDuration(period);          // 30d / 365d / 10000d
    // 3) выполняем команду от имени консоли
    Bukkit.getScheduler().runTask(this, () ->
      Bukkit.dispatchCommand(Bukkit.getConsoleSender(),
        "lp user " + nickname + " parent addtemp " + slug + " " + duration));
  }
}
```

`config.yml`:

```yaml
secret: "ваш-секрет-из-админки-вкладка-API"
```

## Шаг 4 — настроить ESTELAR

В админ-панели → вкладка **API** → блок **Привилегии**:

* `Webhook URL`: `http://ваш-сервер:25580/webhook`
* `Webhook Secret`: тот же секрет, что в `config.yml` плагина

Нажмите «Тестовый запрос» — в консоли сервера появится:

```
[EstelarBridge] Webhook IN event=integration.test cat=privilege ...
```

## Шаг 5 — проверить с реальным заказом

1. Покупатель оформляет заказ на сайте: `nickname=Kuruma`, товар `VIP`.
2. Админ переводит заказ в статус **Выдано**.
3. Backend ESTELAR подписывает payload и отправляет POST на ваш `:25580`.
4. Плагин выполняет:
   ```
   lp user Kuruma parent addtemp vip 10000d
   ```
5. LuckPerms отвечает в консоли:
   ```
   668446a3-2c61-3366-9824-e3b193f823e4 now inherits permissions from vip
   for a duration of 27 years 4 months ... in context global.
   ```

Этот сценарий проверен на стенде PaperMC 1.20.1 + LuckPerms 5.5.42 +
EstelarBridge.jar — лог-файл сервера привожу выше дословно.

## Что важно для offline-серверов

В `server.properties` пиратских серверов обычно `online-mode=false`. Чтобы
LuckPerms не лез к Mojang за UUID и принимал команды по никнейму,
включите:

```yaml
# plugins/LuckPerms/config.yml
use-server-uuid-cache: true
```

Иначе будет `A user for Kuruma could not be found` — это означает, что
LP не смог получить UUID игрока (которого нет в кеше).
