# Frontend: Google Login (openPOS)

Backend base URL: `https://openpos-api.vercel.app/api/v1`
(local: `http://localhost:8080/api/v1`).

## Button setup

```html
<script src="https://accounts.google.com/gsi/client" async></script>
<div
  id="g_id_onload"
  data-client_id="YOUR_GOOGLE_CLIENT_ID.apps.googleusercontent.com"
  data-callback="handleGoogle"
></div>
<div class="g_id_signin" data-type="standard"></div>

<script>
  async function handleGoogle(resp) {
    // resp.credential is the ID token — send it to us, never to Google again.
    const r = await fetch(API_BASE + "/auth/google", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ id_token: resp.credential }),
    });
    const data = await r.json();
    if (!r.ok) throw new Error(data.error);
    saveSession(data); // same shape as password login — see below
  }
</script>
```

(The real Client ID is sent separately in chat — it never lives in this repo.)

Optional: send `"storeName"` alongside `id_token` to name the store created for
brand-new users. Omit it and the backend names it `"<Google name>'s Store"`
(renamable later in Settings).

## Contract

- `POST /auth/google` → `200`
  ```json
  {
    "user": {
      "id": 1,
      "email": "sari@gmail.com",
      "name": "Sari",
      "role": "admin",
      "active": true,
      "store_id": 1,
      "store_name": "Toko Sari"
    },
    "access_token": "eyJ...",
    "refresh_token": "88cb..."
  }
  ```
- New Google email = new store + admin, logged in immediately.
- Known email (OTP *or* Google) = logged into that same account.
  Passwords keep working — both doors stay open.
- Errors: `400 id_token wajib diisi` · `401 login Google tidak valid`
  (bad/expired/wrong-app token) · `400` unverified Google email ·
  `403` account disabled · `500` server missing `GOOGLE_CLIENT_ID`.

## After login: identical to password login

Token storage, refresh interceptor, OTP fallback, cashier switch + PIN pad,
role gating — all as documented in `README.md`. Two gotchas that live here
because they're new:

- **Every id is a JSON number** (`categoryId`, `productId`, `target_user_id`,
  path params). Non-numeric path ids → `400 {"error":"ID tidak valid."}`
- **Key account lists by `role + id`**: admin `1` and cashier `1` are different
  accounts sharing a number. `PUT /users/{id}/passcode` accepts optional
  `"role":"admin"` when targeting the owner on a colliding number.
