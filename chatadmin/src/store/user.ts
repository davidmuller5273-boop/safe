import { defineStore } from 'pinia'
import { ref } from 'vue'
import { current, login } from '@/api'
import type { Admin } from '@/types'
export const useUserStore=defineStore('user',()=>{const token=ref(localStorage.getItem('admin_token')||'');const user=ref<Admin|null>(null);async function signIn(username:string,password:string){const res=await login({username,password});token.value=res.data.token;user.value=res.data.admin;localStorage.setItem('admin_token',token.value)}async function load(){if(!token.value)return false;try{user.value=(await current()).data;return true}catch{return false}}function signOut(){token.value='';user.value=null;localStorage.removeItem('admin_token')}return{token,user,signIn,load,signOut}})
