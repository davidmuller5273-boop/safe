import axios from 'axios'
import { ElMessage } from 'element-plus'

const request = axios.create({ baseURL: import.meta.env.VITE_API_BASE_URL || 'http://localhost:8081', timeout: 10000 })
request.interceptors.request.use(config => { const token=localStorage.getItem('admin_token');if(token)config.headers.Authorization=`Bearer ${token}`;return config })
request.interceptors.response.use(response=>response.data,error=>{if(error.response?.status===401){localStorage.removeItem('admin_token');if(location.pathname!=='/login')location.href='/login'};ElMessage.error(error.response?.data?.message||'请求失败');return Promise.reject(error)})
export default request
