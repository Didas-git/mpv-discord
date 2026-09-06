local msg = require("mp.msg")
local opts = require("mp.options")
local utils = require("mp.utils")

local options = {
    key = "D",
    active = true,
    client_id = "1546028733918609408",
    binary_path = "discord/mpv-discord.exe",
    socket_path = "/tmp/mpvsocket",
    use_static_socket_path = true,
}
opts.read_options(options, "discord")

local script_info = {
    name = "MPV Discord RPC",
    description = "Discord Rich Presence integration for MPV Media Player",
    version = "2.0.0",
}

function file_exists(path) -- fix(#23): use this instead of utils.file_info
    local f = io.open(path, "r")
    if f ~= nil then
        io.close(f)
        return true
    else
        return false
    end
end

local socket_path = options.socket_path
if not options.use_static_socket_path then
    local pid = utils.getpid()
    local filename = ("mpv-discord-%s"):format(pid)
    if socket_path == "" then
        socket_path = "/tmp/" -- default
    end
    socket_path = utils.join_path(socket_path, filename)
elseif socket_path == "" then
    msg.fatal("Missing socket path in config file.")
    os.exit(1)
end
msg.info(("(mpv-ipc): %s"):format(socket_path))
mp.set_property("input-ipc-server", socket_path)

local cmd = nil

-- mp.register_event("file-loaded", start)

local function start()
    if not options.active then return end

    if cmd == nil then
        cmd = mp.command_native_async({
            name = "subprocess",
            playback_only = false,
            args = {
                options.binary_path,
                socket_path,
                options.client_id,
            },
        }, function() end)
        msg.info("launched subprocess")
    end
end

function stop()
    mp.abort_async_command(cmd)
    cmd = nil
    msg.info("aborted subprocess")
end

mp.add_key_binding(options.key, "toggle-discord", function()
    local status = "Active"
    if cmd ~= nil then
        stop()
        status = "Inactive"
    else
        start()
    end
    mp.osd_message(("[%s] Status: %s"):format(script_info.name, status))
    msg.info(string.format("Status: %s", status))
end, { repeatable = false })

mp.register_event("shutdown", function()
    if cmd ~= nil then
        stop()
    end
    if not options.use_static_socket_path then
        os.remove(socket_path)
    end
end)

-- print script info
msg.info(string.format(script_info.description))
msg.info(string.format("Version: %s", script_info.version))

start()
