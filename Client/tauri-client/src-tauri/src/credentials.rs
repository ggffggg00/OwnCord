//! Cross-platform credential storage (Windows Credential Manager, macOS Keychain,
//! Secret Service on Linux) via the `keyring` crate.
//!
//! Target name: service `OwnCord`, account name = host string.
//! Secret: JSON `{"username":"...","token":"...","password":"..."}`.

use keyring::Entry;
use serde::Serialize;

/// Data returned from `load_credential`.
#[derive(Serialize, Clone)]
pub struct CredentialData {
    pub username: String,
    pub token: String,
    // Password is stored in the credential blob for re-authentication but
    // is never serialized back to the frontend over IPC to limit exposure.
    #[serde(skip)]
    pub password: Option<String>,
}

impl std::fmt::Debug for CredentialData {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        f.debug_struct("CredentialData")
            .field("username", &self.username)
            .field("token", &"[REDACTED]")
            .field("password", &self.password.as_ref().map(|_| "[REDACTED]"))
            .finish()
    }
}

fn entry_for_host(host: &str) -> Result<Entry, String> {
    Entry::new("OwnCord", host).map_err(|e| format!("keyring entry: {e}"))
}

fn is_not_found(err: &keyring::Error) -> bool {
    matches!(err, keyring::Error::NoEntry)
}

/// Save a credential (username + token + optional password).
///
/// The optional password field is only included when the user checks "Remember password".
#[tauri::command]
pub fn save_credential(
    host: String,
    username: String,
    token: String,
    password: Option<String>,
) -> Result<(), String> {
    if host.is_empty() {
        return Err("host must not be empty".into());
    }
    if token.is_empty() {
        return Err("token must not be empty".into());
    }
    if username.is_empty() {
        return Err("username must not be empty".into());
    }

    let mut payload = serde_json::json!({
        "username": username,
        "token": token,
    });
    if let Some(ref pw) = password {
        payload["password"] = serde_json::Value::String(pw.clone());
    }

    let entry = entry_for_host(&host)?;
    entry
        .set_password(&payload.to_string())
        .map_err(|e| format!("store credential: {e}"))
}

/// Load a credential. Returns `None` when no credential exists for the host.
#[tauri::command]
pub fn load_credential(host: String) -> Result<Option<CredentialData>, String> {
    if host.is_empty() {
        return Err("host must not be empty".into());
    }

    let entry = entry_for_host(&host)?;
    let json_str = match entry.get_password() {
        Ok(s) => s,
        Err(ref e) if is_not_found(e) => return Ok(None),
        Err(e) => return Err(format!("read credential: {e}")),
    };

    let parsed: serde_json::Value = serde_json::from_str(&json_str)
        .map_err(|e| format!("credential secret is not valid JSON: {e}"))?;

    let username = parsed
        .get("username")
        .and_then(|v| v.as_str())
        .ok_or("credential missing 'username' field")?
        .to_string();
    let token = parsed
        .get("token")
        .and_then(|v| v.as_str())
        .ok_or("credential missing 'token' field")?
        .to_string();
    let password = parsed
        .get("password")
        .and_then(|v| v.as_str())
        .map(|s| s.to_string());

    Ok(Some(CredentialData {
        username,
        token,
        password,
    }))
}

/// Delete a stored credential for the host.
#[tauri::command]
pub fn delete_credential(host: String) -> Result<(), String> {
    if host.is_empty() {
        return Err("host must not be empty".into());
    }

    let entry = entry_for_host(&host)?;
    match entry.delete_credential() {
        Ok(()) => Ok(()),
        Err(ref e) if is_not_found(e) => Ok(()),
        Err(e) => Err(format!("delete credential: {e}")),
    }
}
