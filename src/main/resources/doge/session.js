(function(sessKey){ //session

  var SetCookie = Java.type('io.helidon.http.SetCookie');
  var hcs = ht.crypto.symmetric;

  var sessionStarted = false;
  var sessionMap = {};
  var sessionKey = '__SESS';
  var charset = java.nio.charset.StandardCharsets.UTF_8;
  cryptoKey = sessKey.getBytes(charset);
  var aes = new hcs.SymmetricCrypto(hcs.SymmetricAlgorithm.AES, cryptoKey);


  var encrypt = function(map) {
    var str = aes.encryptHex(JSON.stringify(map), charset);
    return str;
  };

  var decrypt = function(str) {
    var enc = aes.decryptStr(str, charset);
    return JSON.parse(enc);
  }

  var sendSession = function(req, resp){
    var value = encrypt(sessionMap);
    var cookie = SetCookie
      .builder(sessionKey, value)
      .path('/')
      .sameSite(SetCookie.SameSite.STRICT)
      .httpOnly(true)
      .secure(true)
      .maxAge(java.time.Duration.ofHours(1))
      .get();

    resp.headers().addCookie(cookie);
  };

  var initSession = function($ctx) {
    var sessStr = $ctx.request().headers().cookies().first(sessionKey).orElse(null);
    try {
      sessionMap = decrypt(sessStr);
    }
    catch(e) {
      //ignore
    }
  }

  var self = {
    get: function(key) {
      return sessionMap[key] || null;
    },
    set: function(key, val) {
      sessionMap[key] = val;
    },
    remove: function(key) {
      delete sessionMap[key];
    },
    all: function() {
      return sessionMap;
    }
  };

  return function() {
    if(! sessionStarted) {
      this.beforeSend(sendSession);
      initSession(this);
      sessionStarted = true;
    }

    return self;
  };

});
