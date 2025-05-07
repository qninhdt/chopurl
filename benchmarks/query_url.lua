 -- Seed random number generator per thread
math.randomseed(os.time())

-- List of pre-generated short URLs to query
local short_urls = {
    "5Jh2wuS",
    "5Jh12rd",
    "73wr2qZ",
    "eAwiTBd",
    "19vcneL"
}

request = function()
    -- Randomly select a short URL from the list
    local random_index = math.random(1, #short_urls)
    local short_url = short_urls[random_index]
    
    wrk.method = "GET"
    wrk.path = "/short/" .. short_url
    
    return wrk.format()
end

response = function(status, headers, body)
    if status ~= 200 and status ~= 301 and status ~= 302 then
        print("GET /short/" .. short_url .. " failed with status: " .. status)
    end
end