# BRLN DevDash - API Integration Setup

## 🚀 Quick Start

### 1. Install Python Dependencies

```powershell
cd devdash
pip install flask flask-cors mnemonic
```

### 2. Start Dev API Server

```powershell
python dev_api.py
```

You should see:
```
============================================================
BRLN-OS DevDash API Server
============================================================
Swap Core Available: True/False
Starting on http://localhost:2121
============================================================
```

### 3. Start DevDash Frontend

In a **new terminal**:

```powershell
cd devdash
npm run dev
```

---

## ✅ What's Integrated

| Feature | Status | Endpoint |
|---------|--------|----------|
| **System Status** | ✅ Real Data | `/api/v1/system/status` |
| **Health Metrics** | ✅ Real Data | `/api/v1/system/health` |
| **Wallet Generate** | ✅ Real Data | `/api/v1/wallet/generate` |
| **Preimage Generate** | ✅ Real Data | `/api/v1/preimage/generate` |
| **HTLC Create** | ✅ Real Data (if brln-swap-core available) | `/api/v1/htlc/create` |
| **Lightning Info** | ✅ Mock Data | `/api/v1/lightning/info` |
| **Bitcoin Info** | ✅ Mock Data | `/api/v1/bitcoin/info` |
| **API Explorer** | ✅ Functional | Test all endpoints |

---

## 🧪 Testing

### Test API Explorer

1. Navigate to **API Explorer** page
2. Select any endpoint from the sidebar
3. Click **Send Request**
4. View real response from `dev_api.py`

### Test Home Page

1. Home page now shows **real system status** from API
2. If API is down, automatically falls back to mock data
3. Observability ribbon shows **real health metrics**

---

## 🔧 Optional: Full Integration

To use **real brln-swap-core** for HTLC creation:

1. Ensure `brln-swap-core` is available at `api/brln-swap-core` relative to the repository root.
2. Install dependencies:
   ```powershell
   pip install bitcoin ecdsa
   ```
3. Restart `dev_api.py`

---

## 📝 Notes

- **Fallback Strategy**: All components gracefully fallback to mock data if API is unavailable
- **CORS Enabled**: API accepts requests from `localhost:3000`
- **In-Memory Storage**: Events and wallets stored in memory (resets on restart)
- **Real Wallet Generation**: Uses `mnemonic` library for BIP39 seed phrases
