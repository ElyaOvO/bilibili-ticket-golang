import { defineStore } from 'pinia'
import { ref } from 'vue'
import { GetAccountStatus, CheckAndUpdateCookie, PersistCookies, SetRefreshToken } from '../../bindings/bilibili-ticket-golang/cmd/gui/app'
import type * as api from '../../bindings/bilibili-ticket-golang/lib/models/bili/api/models'

export const useAuthStore = defineStore('auth', () => {
    const isLogin = ref(false)
    const username = ref('')
    const uid = ref(0)
    const checked = ref(false)

    async function checkLoginStatus(): Promise<boolean> {
        try {
            const status: api.GetLoginInfoStruct | null = await GetAccountStatus()
            if (!status) {
                checked.value = true
                return false
            }
            isLogin.value = status.isLogin
            if (status.isLogin) {
                username.value = status.uname || ''
                uid.value = status.mid || 0
                // Refresh cookie if needed, then persist
                CheckAndUpdateCookie()
                    .then(() => PersistCookies())
                    .catch(() => { })
            }
            checked.value = true
            return status.isLogin
        } catch {
            checked.value = true
            return false
        }
    }

    function setLoggedIn(name: string, mid: number) {
        isLogin.value = true
        username.value = name
        uid.value = mid
    }

    /**
     * Save the refresh_token from QR login response and trigger persistence.
     * Must be called after a successful QR login (code === 0).
     */
    async function saveRefreshToken(token: string) {
        await SetRefreshToken(token)
        await PersistCookies()
    }

    return { isLogin, username, uid, checked, checkLoginStatus, setLoggedIn, saveRefreshToken }
})
