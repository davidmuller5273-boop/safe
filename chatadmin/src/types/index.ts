export interface Permission { id:number; code:string; name:string; description:string }
export interface Role { id:number; name:string; description:string; permissions:Permission[] }
export interface Admin { id:number; username:string; name:string; enabled:boolean; role_id:number; role_name:string; permissions:string[]; created_at:string }
export interface LotteryType { id:number; name:string; symbol:string; seconds_per_issue:number; issues_per_day:number; draws_all_day:boolean; created_at:string; updated_at:string }
export interface DrawRecord { id:number; lottery_type_id:number; lottery_type:LotteryType; issue_number:string; draw_result:string; draw_timestamp:number; next_draw_timestamp:number; next_issue_number:string; created_at:string; updated_at:string }
export interface PageData<T> { items:T[]; total:number; page:number; page_size:number }
export interface SystemConfig { safew_bot_token_configured:boolean; safew_bot_token_masked:string; safew_chat_ids:string[]; safew_bot_ready:boolean }
export interface ApiResponse<T> { code:number; message:string; data:T }
