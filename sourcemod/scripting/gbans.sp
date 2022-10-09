#pragma semicolon 1
#pragma tabsize 4
#pragma newdecls required

#include <basecomm>
#include <json> // sm-json
#include <sdktools>
#include <sourcemod>
#include <system2> // system2 extension
#include <gbans>

#define DEBUG

#define PLUGIN_AUTHOR "Leigh MacDonald"
#define PLUGIN_VERSION "0.00"
#define PLUGIN_NAME "gbans"

// Authentication token len
#define TOKEN_LEN 40

#define HTTP_STATUS_OK 200
#define HTTP_STATUS_CONFLICT 409

#define PERMISSION_RESERVED 15
#define PERMISSION_EDITOR 25
#define PERMISSION_MOD 50
#define PERMISSION_ADMIN 100

#define FLAGS_RESERVED "a"
#define FLAGS_EDITOR "aj"
#define FLAGS_MOD "abcdegjk"
#define FLAGS_ADMIN "z"

// clang-format off
enum struct PlayerInfo {
    bool authed;
    char ip[16];
    int ban_type;
    int permission_level;
    char message[256];
}
// clang-format on

// Globals must all start with g_
char g_token[TOKEN_LEN + 1]; // tokens are 40 chars + term

PlayerInfo g_players[MAXPLAYERS + 1];

int g_port;
char g_host[128];
char g_server_name[128];
char g_server_key[41];

// Store temp clientId for networked callbacks 
int g_reply_to_client_id = 0;

public
Plugin myinfo = {name = PLUGIN_NAME, author = PLUGIN_AUTHOR, description = "gbans game client",
                 version = PLUGIN_VERSION, url = "https://github.com/leighmacdonald/gbans"};

public
void OnPluginStart() {
    LoadTranslations("common.phrases.txt");

    RegConsoleCmd("gb_version", CmdVersion, "Get gbans version");
    RegConsoleCmd("gb_mod", CmdMod, "Ping a moderator");
    RegConsoleCmd("mod", CmdMod, "Ping a moderator");
    RegAdminCmd("gb_ban", AdminCmdBan, ADMFLAG_BAN);
    //RegAdminCmd("gb_banip", AdminCmdBanIP, ADMFLAG_BAN);
    RegAdminCmd("gb_reauth", AdminCmdReauth, ADMFLAG_KICK);
    RegConsoleCmd("gb_help", CmdHelp, "Get a list of gbans commands");

    ReadConfig();
    AuthenticateServer();
}

public APLRes AskPluginLoad2(Handle myself, bool late, char[] error, int err_max)
{
	CreateNative("GB_BanClient", Native_GB_BanClient);
	return APLRes_Success;
}

void ReadConfig() {
    char localPath[PLATFORM_MAX_PATH];
    BuildPath(Path_SM, localPath, sizeof(localPath), "configs/%s", "gbans.cfg");
#if defined DEBUG
    PrintToServer("[GB] Using config file: %s", localPath);
#endif
    KeyValues kv = new KeyValues("gbans");
    if (!kv.ImportFromFile(localPath)) {
        PrintToServer("[GB] No config file could be found");
    } else {
        kv.GetString("host", g_host, sizeof(g_host), "http://localhost");
        g_port = kv.GetNum("port", 6006);
        kv.GetString("server_name", g_server_name, sizeof(g_server_name), "default");
        kv.GetString("server_key", g_server_key, sizeof(g_server_key), "");
    }
    delete kv;
}

System2HTTPRequest newReq(System2HTTPResponseCallback cb, const char[] path) {
    char fullAddr[1024];
    Format(fullAddr, sizeof(fullAddr), "%s%s", g_host, path);
    System2HTTPRequest httpRequest = new System2HTTPRequest(cb, fullAddr);
    httpRequest.SetPort(g_port);
    httpRequest.SetHeader("Content-Type", "application/json");
    if (strlen(g_token) == TOKEN_LEN) {
        httpRequest.SetHeader("Authorization", g_token);
    }
    return httpRequest;
}

public
void OnClientPostAdminCheck(int clientId) {
    switch (g_players[clientId].ban_type) {
        case BSNoComm: {
            if (!BaseComm_IsClientMuted(clientId)) {
                BaseComm_SetClientMute(clientId, true);
            }
            if (!BaseComm_IsClientGagged(clientId)) {
                BaseComm_SetClientGag(clientId, true);
            }
            ReplyToCommand(clientId, "You are currently muted/gag, it will expire automatically");
            LogAction(0, clientId, "Muted \"%L\" for an unfinished mute punishment.", clientId);
        }
        case BSBanned: {
            KickClient(clientId, g_players[clientId].message);
            LogAction(0, clientId, "Kicked \"%L\" for an unfinished ban.", clientId);
        }
    }
}

/**
Authenicates the server with the backend API system.

Send unauthenticated request for token to -> API /api/server_auth
Recv Token <- API
Send authenticated commands with header "Authorization $token" set for subsequent calls -> API /api/<path>

*/
void AuthenticateServer() {
    JSON_Object obj = new JSON_Object();
    obj.SetString("server_name", g_server_name);
    obj.SetString("key", g_server_key);
    char encoded[1024];
    obj.Encode(encoded, sizeof(encoded));
    json_cleanup_and_delete(obj);

    System2HTTPRequest req = newReq(OnAuthReqReceived, "/api/server_auth");
    req.SetData(encoded);
    req.POST();
    delete req;
}

void OnAuthReqReceived(bool success, const char[] error, System2HTTPRequest request, System2HTTPResponse response,
                       HTTPRequestMethod method) {
    if (success) {
        char lastURL[128];
        response.GetLastURL(lastURL, sizeof(lastURL));
        int statusCode = response.StatusCode;
        float totalTime = response.TotalTime;
#if defined DEBUG
        PrintToServer("[GB] Request to %s finished with status code %d in %.2f seconds", lastURL, statusCode,
                      totalTime);
#endif
        if (statusCode != HTTP_STATUS_OK) {
            PrintToServer("[GB] Bad status on authentication request: %d", statusCode);
            return;
        }
        char[] content = new char[response.ContentLength + 1];
        response.GetContent(content, response.ContentLength + 1);
        JSON_Object resp = json_decode(content);
        bool ok = resp.GetBool("status");
        if (!ok) {
            PrintToServer("[GB] Invalid response status, cannot authenticate");
            return;
        }
        JSON_Object data = resp.GetObject("result");
        char token[41];
        data.GetString("token", token, sizeof(token));
        if (strlen(token) != 40) {
            PrintToServer("[GB] Invalid response status, invalid token");
            return;
        }
        g_token = token;
        PrintToServer("[GB] Successfully authenticated with gbans server");
        json_cleanup_and_delete(resp);
    } else {
        PrintToServer("[GB] Error on authentication request: %s", error);
    }
}


public
Action AdminCmdReauth(int clientId, int argc) {
    AuthenticateServer();
    return Plugin_Handled;
}

// public
// Action AdminCmdKick(int clientId, int argc) {
//     if (IsClientInGame(clientId) && !IsFakeClient(clientId)) {
//         KickClient(clientId);
//     }
//     return Plugin_Handled;
// }

public
Action CmdVersion(int clientId, int args) {
    ReplyToCommand(clientId, "[GB] Version %s", PLUGIN_VERSION);
    return Plugin_Handled;
}

/**
Ping the moderators through discord
*/
public
Action CmdMod(int clientId, int argc) {
    if (argc < 1) {
        ReplyToCommand(clientId, "Must supply a reason message for pinging");
        return Plugin_Handled;
    }
    char reason[256];
    for (int i = 1; i <= argc; i++) {
        if (i > 1) {
            StrCat(reason, sizeof(reason), " ");
        }
        char buff[128];
        GetCmdArg(i, buff, sizeof(buff));
        StrCat(reason, sizeof(reason), buff);
    }
    char auth_id[50];
    if (!GetClientAuthId(clientId, AuthId_Steam3, auth_id, sizeof(auth_id), true)) {
        ReplyToCommand(clientId, "Failed to get auth_id of user: %d", clientId);
        return Plugin_Continue;
    }
    char name[64];
    if (!GetClientName(clientId, name, sizeof(name))) {
        PrintToServer("Failed to get user name?");
        return Plugin_Continue;
    }
    JSON_Object obj = new JSON_Object();
    obj.SetString("server_name", g_server_name);
    obj.SetString("steam_id", auth_id);
    obj.SetString("name", name);
    obj.SetString("reason", reason);
    obj.SetInt("client", clientId);
    char encoded[1024];
    obj.Encode(encoded, sizeof(encoded));
    json_cleanup_and_delete(obj);
    System2HTTPRequest req = newReq(OnPingModRespReceived, "/api/ping_mod");
    req.SetData(encoded);
    req.POST();
    delete req;

    ReplyToCommand(clientId, "Mods have been alerted, thanks!");

    return Plugin_Handled;
}

void OnPingModRespReceived(bool success, const char[] error, System2HTTPRequest request, System2HTTPResponse response,
                           HTTPRequestMethod method) {
    if (!success) {
        return;
    }
    if (response.StatusCode != HTTP_STATUS_OK) {
        PrintToServer("[GB] Bad status on mod resp request (%d): %s", response.StatusCode, error);
        return;
    }
}

public
Action CmdHelp(int clientId, int argc) {
    CmdVersion(clientId, argc);
    ReplyToCommand(clientId, "gb_ban #user duration [reason]");
    ReplyToCommand(clientId, "gb_ban_ip #user duration [reason]");
    ReplyToCommand(clientId, "gb_kick #user [reason]");
    ReplyToCommand(clientId, "gb_mute #user duration [reason]");
    ReplyToCommand(clientId, "gb_mod reason");
    ReplyToCommand(clientId, "gb_version -- Show the current version");
    return Plugin_Handled;
}

public
bool OnClientConnect(int clientId, char[] rejectMsg, int maxLen) {
    g_players[clientId].authed = false;
    g_players[clientId].ban_type = BSUnknown;
    return true;
}

public
void OnClientAuthorized(int clientId, const char[] auth) {
    char ip[16];
    GetClientIP(clientId, ip, sizeof(ip));

    char name[32];
    GetClientName(clientId, name, sizeof(name));

    /* Do not check bots nor check player with lan steamid. */
    if (auth[0] == 'B' /*|| auth[9] == 'L'*/) {
        g_players[clientId].authed = true;
        g_players[clientId].ip = ip;
        g_players[clientId].ban_type = BSUnknown;
        return;
    }
#if defined DEBUG
    PrintToServer("[GB] Checking ban state for: %s", auth);
#endif
    CheckPlayer(clientId, auth, ip, name);
}

any Native_GB_BanClient(Handle plugin, int numParams) {
    int adminId = GetNativeCell(1);
    if (adminId <= 0) {
        return ThrowNativeError(SP_ERROR_NATIVE, "Invalid adminId index (%d)", adminId);
    }
    int targetId = GetNativeCell(2);
    if (targetId <= 0) {
        return ThrowNativeError(SP_ERROR_NATIVE, "Invalid targetId index (%d)", targetId);
    }
    int reason = GetNativeCell(3);
    if (reason <= 0) {
        return ThrowNativeError(SP_ERROR_NATIVE, "Invalid reason index (%d)", reason);
    }
    char duration[32]; 
    if ( GetNativeString(4, duration, sizeof(duration)) != SP_ERROR_NONE) {
        return ThrowNativeError(SP_ERROR_NATIVE, "Invalid duration, but must be positive integer or 0 for permanent");
    }
    int banType = GetNativeCell(5);
    if (banType != BSBanned && banType != BSNoComm) {
        return ThrowNativeError(SP_ERROR_NATIVE, "Invalid banType, but must be 1: mute/gag  or 2: ban");
    }
    banReason reasonValue = view_as<banReason>(reason);
    if (!ban(adminId, targetId, reasonValue, duration, banType)) {
        return ThrowNativeError(SP_ERROR_NATIVE, "Ban error ");
    }
    return true;
}

public
Action AdminCmdBan(int clientId, int argc) {
    char command[64];
    char targetIdStr[50];
    char duration[50];
    char banTypeStr[50];
    char reasonStr[256];
    char usage[] = "Usage: %s <targetId> <banType> <duration> <reason>";

    GetCmdArg(0, command, sizeof(command));

    if (argc < 4) {
        ReplyToCommand(clientId, usage, command);
        return Plugin_Handled;
    }

    GetCmdArg(1, targetIdStr, sizeof(targetIdStr));
    GetCmdArg(2, banTypeStr, sizeof(banTypeStr));
    GetCmdArg(3, duration, sizeof(duration));
    GetCmdArg(4, reasonStr, sizeof(reasonStr));

    PrintToServer("Target: %s banType: %s duration: %s reason: %s", targetIdStr, banTypeStr, duration, reasonStr);

    int targetIdx = FindTarget(clientId, targetIdStr, true, false);
    if (targetIdx < 0) {
        ReplyToCommand(clientId, "Failed to locate user: %s", targetIdStr);
        return Plugin_Handled;
    }
    banReason reason = custom;
    if (!parseReason(reasonStr, reason)) {
        ReplyToCommand(clientId, "Failed to parse reason");
        return Plugin_Handled;
    }
    int banType = StringToInt(banTypeStr);
    if (banType != BSNoComm && banType != BSBanned) {
        ReplyToCommand(clientId, "Invalid ban type");
        return Plugin_Handled;
    }
    
    if (!ban(clientId, targetIdx, reason, duration, banType)) {
        ReplyToCommand(clientId, "Error sending ban request");
    }

    return Plugin_Handled;
}

/**
 * ban performs the actual work of sending the ban request to the gbans server
 * 
 * NOTE: There is currently no way to set a custom ban reason string
 */
public
bool ban(int sourceId, int targetId, banReason reason, const char[] duration, int banType)  {
    char sourceSid[50];
    if (!GetClientAuthId(sourceId, AuthId_Steam3, sourceSid, sizeof(sourceSid), true)) {
        ReplyToCommand(sourceId, "Failed to get sourceId of user: %d", sourceId);
        return false;
    }
    char targetSid[50];
    if (!GetClientAuthId(targetId, AuthId_Steam3, targetSid, sizeof(targetSid), true)) {
        ReplyToCommand(sourceId, "Failed to get targetId of user: %d", targetId);
        return false;
    }

    JSON_Object obj = new JSON_Object();
    obj.SetString("source_id", sourceSid);
    obj.SetString("target_id", targetSid);
    obj.SetString("note", "");
    obj.SetString("reason_text", "");
    obj.SetInt("ban_type", banType);
    obj.SetInt("reason", view_as<int>(reason));
    obj.SetString("duration", duration);
    obj.SetInt("report_id", 0);

    char encoded[1024];
    obj.Encode(encoded, sizeof(encoded));
    json_cleanup_and_delete(obj);
    System2HTTPRequest req = newReq(OnBanRespReceived, "/api/sm/bans/steam/create");
    req.SetData(encoded);
    req.POST();
    delete req;

    g_reply_to_client_id = sourceId;

    return true;
}

void OnBanRespReceived(bool success, const char[] error, System2HTTPRequest request, System2HTTPResponse response,
                           HTTPRequestMethod method) {
    if (!success) {
        PrintToServer("[GB] Ban request did not complete successfully");
        return;
    }

    if (response.StatusCode != HTTP_STATUS_OK) {
        if (response.StatusCode == HTTP_STATUS_CONFLICT) {
            ReplyToCommand(g_reply_to_client_id, "Duplicate ban");
            return;
        }
        ReplyToCommand(g_reply_to_client_id, "Unhandled error response");
        return;
    }

    char[] content = new char[response.ContentLength + 1];
    
    response.GetContent(content, response.ContentLength + 1);

    JSON_Object resp = json_decode(content);
    if (!resp.GetBool("status")) {
        PrintToServer("[GB] Invalid response status");
        json_cleanup_and_delete(resp);
        return;
    }

    JSON_Object banResult = resp.GetObject("result");
    int banId = banResult.GetInt("ban_id");
    ReplyToCommand(g_reply_to_client_id, "User banned (#%d)", banId);

    json_cleanup_and_delete(resp);
}

public
bool parseReason(const char[] reasonStr, banReason &reason) {
    int reasonInt = StringToInt(reasonStr, 10);
    if (reasonInt <= 0 || reasonInt < view_as<int>(custom) || reasonInt > view_as<int>(itemDescriptions)) {
        return false;
    }
    reason = view_as<banReason>(reasonInt);
    return true;
}


void CheckPlayer(int clientId, const char[] auth, const char[] ip, const char[] name) {
    if (/**!IsClientConnected(clientId) ||*/ IsFakeClient(clientId)) {
        PrintToServer("[GB] Skipping check on invalid player");
        return;
    }
    char encoded[1024];
    JSON_Object obj = new JSON_Object();
    obj.SetString("steam_id", auth);
    obj.SetInt("client_id", clientId);
    obj.SetString("ip", ip);
    obj.SetString("name", name);
    obj.Encode(encoded, sizeof(encoded));
    json_cleanup_and_delete(obj);

    System2HTTPRequest req = newReq(OnCheckResp, "/api/check");
    req.SetData(encoded);
    req.POST();
    delete req;
}

void OnCheckResp(bool success, const char[] error, System2HTTPRequest request, System2HTTPResponse response,
                 HTTPRequestMethod method) {
    if (success) {
        char lastURL[128];
        response.GetLastURL(lastURL, sizeof(lastURL));
        int statusCode = response.StatusCode;
        float totalTime = response.TotalTime;
#if defined DEBUG
        PrintToServer("[GB] Request to %s finished with status code %d in %.2f seconds", lastURL, statusCode,
                      totalTime);
#endif
        char[] content = new char[response.ContentLength + 1];
        response.GetContent(content, response.ContentLength + 1);
        if (statusCode != HTTP_STATUS_OK) {
            // Fail open if the server is broken
            return;
        }
        
        JSON_Object resp = json_decode(content);
        JSON_Object data = resp.GetObject("result");
        int client_id = data.GetInt("client_id");
        int ban_type = data.GetInt("ban_type");
        int permission_level = data.GetInt("permission_level");
        char msg[256]; // welcome or ban message
        data.GetString("msg", msg, sizeof(msg));
        if(IsFakeClient(client_id)) {
            return;
        }
        char ip[16];
        GetClientIP(client_id, ip, sizeof(ip));
        g_players[client_id].authed = true;
        g_players[client_id].ip = ip;
        g_players[client_id].ban_type = ban_type;
        g_players[client_id].message = msg;
        g_players[client_id].permission_level = permission_level;
        char identity[50];
        GetClientAuthId(client_id, AuthId_Steam3, identity, sizeof(identity), true);

        // Anyone with special priviledges is considered an admin
        bool is_admin = permission_level >= PERMISSION_RESERVED;
        if (is_admin && FindAdminByIdentity("steam", identity) == INVALID_ADMIN_ID) {
            char name[50];
            if (!GetClientName(client_id, name, sizeof(name))) {
                PrintToServer("Unable to get client name", name, identity);
                return;
            }
            
            AdminId adminId = CreateAdmin(name);
            switch (permission_level) {
                case PERMISSION_ADMIN:{
                    adminId.SetFlag(Admin_Root, true);
                }
                case PERMISSION_MOD: {
                    adminId.SetFlag(Admin_Reservation, true);
                    adminId.SetFlag(Admin_Generic, true);
                    adminId.SetFlag(Admin_Kick, true);
                    adminId.SetFlag(Admin_Ban, true);
                }
                case PERMISSION_EDITOR: {
                    adminId.SetFlag(Admin_Reservation, true);
                    adminId.SetFlag(Admin_Generic, true);
                    adminId.SetFlag(Admin_Kick, true);
                }
                case PERMISSION_RESERVED: {
                    adminId.SetFlag(Admin_Reservation, true);
                }

            }
            
        }


        PrintToServer("[GB] Client authenticated (banType: %d level: %d)", ban_type, permission_level);
        
        json_cleanup_and_delete(resp);  
    } else {
        PrintToServer("[GB] Error on authentication request: %s", error);
    }
}
