export interface Permission { id:number; code:string; name:string; description:string }
export interface Role { id:number; name:string; description:string; permissions:Permission[] }
export interface Admin { id:number; username:string; name:string; enabled:boolean; role_id:number; role_name:string; permissions:string[]; created_at:string }
export interface LotteryType { id:number; name:string; symbol:string; seconds_per_issue:number; issues_per_day:number; draws_all_day:boolean; created_at:string; updated_at:string }
export interface DrawRecord { id:number; lottery_type_id:number; lottery_type:LotteryType; issue_number:string; draw_result:string; draw_timestamp:number; next_draw_timestamp:number; next_issue_number:string; created_at:string; updated_at:string }
export interface PageData<T> { items:T[]; total:number; page:number; page_size:number }
export interface SystemConfig { safew_bot_token_configured:boolean; safew_bot_token_masked:string; safew_chat_ids:string[]; safew_bot_ready:boolean }
export interface ApiResponse<T> { code:number; message:string; data:T }
export interface LotteryBroadcastGame { code:string; name:string; source:string; enabled:boolean }
export interface LotteryBroadcastLatest { game_code:string; game_name:string; issue:string; draw_time:string; numbers:string; source:string; fetched_at:string }
export interface LotteryBroadcastSource { source:string; label:string; last_checked_at:string; last_success_at:string|null; last_error:string }
export interface LotteryBroadcastSubscription { id:number; chat_id:string; selector:string; label:string; created_by:string; is_enabled:number; created_at:string; group_title:string }
export interface LotteryBroadcastOutbox { id:number; chat_id:string; game_code:string; game_name:string; issue:string; status:string; attempts:number; last_error:string; message_id:number; pinned:number; created_at:string; sent_at:string|null }
export interface LotteryBroadcastOverview { query_enabled:boolean; broadcast_enabled:boolean; with_ads:boolean; games:LotteryBroadcastGame[]; latest:LotteryBroadcastLatest[]; sources:LotteryBroadcastSource[]; subscriptions:LotteryBroadcastSubscription[]; outbox:LotteryBroadcastOutbox[] }
