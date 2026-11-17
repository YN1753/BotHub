import 'webauthn-polyfills'

export interface PasskeyCredentialItem {
  id: string
  name: string
  aaguid: string
  credentialDeviceType: 'singleDevice' | 'multiDevice' | string
  credentialBackedUp: boolean
  transports: string[]
  registeredAt: number
  lastUsedAt?: number
}

export interface PasskeyCapabilities {
  platformSupported: boolean
  conditionalSupported: boolean
}

export function isValidBase64Url(value: unknown): value is string {
  return (
    typeof value === 'string' &&
    value.length > 0 &&
    value.length % 4 !== 1 &&
    /^[A-Za-z0-9_-]+$/.test(value)
  )
}

export async function checkPasskeyCapabilities(): Promise<PasskeyCapabilities> {
  if (typeof window === 'undefined' || !window.PublicKeyCredential) {
    return { platformSupported: false, conditionalSupported: false }
  }
  const PKC = window.PublicKeyCredential as any
  if (typeof PKC.getClientCapabilities === 'function') {
    try {
      const caps = await PKC.getClientCapabilities()
      return {
        platformSupported: Boolean(caps?.passkeyPlatformAuthenticator),
        conditionalSupported: Boolean(caps?.conditionalGet),
      }
    } catch {
      // Fallback below
    }
  }
  try {
    const platform =
      typeof window.PublicKeyCredential.isUserVerifyingPlatformAuthenticatorAvailable === 'function'
        ? await window.PublicKeyCredential.isUserVerifyingPlatformAuthenticatorAvailable()
        : false
    const conditional =
      typeof window.PublicKeyCredential.isConditionalMediationAvailable === 'function'
        ? await window.PublicKeyCredential.isConditionalMediationAvailable()
        : false
    return { platformSupported: platform, conditionalSupported: conditional }
  } catch {
    return { platformSupported: false, conditionalSupported: false }
  }
}

export async function syncAcceptedPasskeys(
  rpId: string,
  userId: string,
  credentials: PasskeyCredentialItem[]
): Promise<void> {
  if (typeof window === 'undefined' || !window.PublicKeyCredential) return
  const PKC = window.PublicKeyCredential as any
  if (typeof PKC.signalAllAcceptedCredentials !== 'function') return

  const credentialIds = credentials.map((c) => c.id)
  if (!rpId || !isValidBase64Url(userId) || !credentialIds.every(isValidBase64Url)) {
    console.warn('SignalAllAcceptedCredentials skipped: invalid rpId or Base64URL identifiers.')
    return
  }
  try {
    await PKC.signalAllAcceptedCredentials({
      rpId,
      userId,
      allAcceptedCredentialIds: credentialIds,
    })
  } catch (err) {
    console.warn('SignalAllAcceptedCredentials sync warning:', err)
  }
}

export async function registerPasskeyCeremony(options: {
  promotion?: boolean
  name?: string
} = {}): Promise<{ ok: boolean; cancelled?: boolean; error?: string; credential?: PasskeyCredentialItem }> {
  const optRes = await fetch('/api/auth/passkey/register/options', {
    method: 'POST',
    credentials: 'same-origin',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ promotion: Boolean(options.promotion) }),
  })
  const optJson = await optRes.json().catch(() => ({}))
  if (!optRes.ok || !optJson?.publicKey || !optJson?.ceremony_id) {
    return { ok: false, error: optJson?.error || '获取通行密钥挑战失败' }
  }

  const PKC = window.PublicKeyCredential as any
  const serverRpId: string = optJson.publicKey?.rp?.id || ''
  const publicKey = PKC.parseCreationOptionsFromJSON(optJson.publicKey)

  let credential: any
  try {
    credential = await navigator.credentials.create({ publicKey })
  } catch (err: any) {
    if (err?.name === 'InvalidStateError') {
      return { ok: false, error: '当前设备密码库中已注册过通行密钥' }
    }
    if (err?.name === 'SecurityError') {
      return { ok: false, error: '通行密钥要求使用 HTTPS 域名或 localhost 访问' }
    }
    if (err?.name === 'NotAllowedError' || err?.name === 'AbortError') {
      return { ok: false, cancelled: true }
    }
    return { ok: false, error: err?.message || '调起面容/指纹验证失败' }
  }

  if (!credential) {
    return { ok: false, cancelled: true }
  }

  const encodedResponse = typeof credential.toJSON === 'function' ? credential.toJSON() : credential
  const qs = new URLSearchParams({ ceremony_id: optJson.ceremony_id })
  if (options.name?.trim()) {
    qs.set('name', options.name.trim())
  }

  try {
    const verifyRes = await fetch(`/api/auth/passkey/register/verify?${qs.toString()}`, {
      method: 'POST',
      credentials: 'same-origin',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(encodedResponse),
    })
    const verifyJson = await verifyRes.json().catch(() => ({}))
    if (!verifyRes.ok) {
      if (typeof PKC.signalUnknownCredential === 'function' && serverRpId && isValidBase64Url(encodedResponse?.id)) {
        try {
          await PKC.signalUnknownCredential({
            rpId: serverRpId,
            credentialId: encodedResponse.id,
          })
        } catch (sigErr) {
          console.warn('SignalUnknownCredential warning:', sigErr)
        }
      }
      return { ok: false, error: verifyJson?.error || '服务端校验通行密钥失败' }
    }
    return { ok: true, credential: verifyJson.credential }
  } catch (netErr: any) {
    if (typeof PKC.signalUnknownCredential === 'function' && serverRpId && isValidBase64Url(encodedResponse?.id)) {
      try {
        await PKC.signalUnknownCredential({
          rpId: serverRpId,
          credentialId: encodedResponse.id,
        })
      } catch (sigErr) {
        console.warn('SignalUnknownCredential warning:', sigErr)
      }
    }
    return { ok: false, error: netErr?.message || '网络异常，保存通行密钥失败' }
  }
}

export async function loginWithPasskeyCeremony(options: {
  signal?: AbortSignal
  conditional?: boolean
} = {}): Promise<{ ok: boolean; cancelled?: boolean; error?: string }> {
  const optRes = await fetch('/api/auth/passkey/login/options', {
    method: 'POST',
    credentials: 'same-origin',
  })
  const optJson = await optRes.json().catch(() => ({}))
  if (!optRes.ok || !optJson?.publicKey || !optJson?.ceremony_id) {
    return { ok: false, error: optJson?.error || '无法发起生物识别验证' }
  }

  const PKC = window.PublicKeyCredential as any
  const serverRpId: string = optJson.publicKey?.rpId || ''
  const publicKey = PKC.parseRequestOptionsFromJSON(optJson.publicKey)

  let credential: any
  try {
    credential = await navigator.credentials.get({
      publicKey,
      signal: options.signal,
      ...(options.conditional ? { mediation: 'conditional' as CredentialMediationRequirement } : {}),
    })
  } catch (err: any) {
    if (err?.name === 'NotAllowedError' || err?.name === 'AbortError') {
      return { ok: false, cancelled: true }
    }
    return { ok: false, error: err?.message || '生物识别验证未完成' }
  }

  if (!credential) {
    return { ok: false, cancelled: true }
  }

  const encoded = typeof credential.toJSON === 'function' ? credential.toJSON() : credential
  try {
    const verifyRes = await fetch(
      `/api/auth/passkey/login/verify?ceremony_id=${encodeURIComponent(optJson.ceremony_id)}`,
      {
        method: 'POST',
        credentials: 'same-origin',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(encoded),
      }
    )
    const verifyJson = await verifyRes.json().catch(() => ({}))
    if (!verifyRes.ok) {
      if (
        verifyRes.status === 404 &&
        typeof PKC.signalUnknownCredential === 'function' &&
        serverRpId &&
        isValidBase64Url(encoded?.id)
      ) {
        try {
          await PKC.signalUnknownCredential({
            rpId: serverRpId,
            credentialId: encoded.id,
          })
        } catch (sigErr) {
          console.warn('SignalUnknownCredential warning:', sigErr)
        }
      }
      return { ok: false, error: verifyJson?.error || '生物识别签名未通过验证' }
    }
    return { ok: true }
  } catch (netErr: any) {
    return { ok: false, error: netErr?.message || '网络异常，验证中断' }
  }
}
