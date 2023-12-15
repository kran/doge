(function($request, $response, $config) {
  var _M = {};

  $config = $config || {};

  var HtmlSource = Java.type('net.htmlparser.jericho.Source');

  load(`classpath:doge/dot.js`);

  const API_DIR = $config.apiDir || 'app/api';
  const THEME_DIR = $config.themeDir || 'app/theme';

  {
    var routerParts = $request.path().path().replace("/", "").split('/');
    _M.filePath = routerParts.shift() || 'index';
    _M.routerPath = routerParts.shift() || 'index';
    _M.extraPaths = routerParts;
  }

  var beforeSendHooks = [];
  _M.beforeSend = function(fn) {
    beforeSendHooks.push(fn);
    return _M;
  }

  _M.ensureRouteFile = function() {
    var root = new File(`${API_DIR}`);
    var file = new File(root, `${_M.filePath}.js`);
    if(file.exists() && file.getCanonicalPath().startsWith(root.getCanonicalPath())) {
      return file;
    }

    throw new SiteError(404, '无法找到该页面');
  }

  _M.route = function(routerFactory) {
    var routers = routerFactory(_M);
    var path = _M.routerPath;
    var fn = routers[path] || routers['$fallback'];

    if(Object.prototype.toString.call(fn) === "[object Function]") {
      return fn.apply(_M, _M.extraPaths);
    }
    else {
      throw new SiteError(404, '无此路由');
    }
  };

  _M.render = function(tpl, dataMap, type) {
    var data = {"$ctx": _M, render: _M.render};
    for(var dataKey in dataMap) {
      data[dataKey] = dataMap[dataKey];
    }
    var fileParts = tpl.match(/^(.+?)(?:(#|@)(.+))?$/);
    var path = new File(THEME_DIR, fileParts[1]);
    var fileStr = readFully(path);

    if(fileParts[2]) {
      var source = new HtmlSource(fileStr);
      var dom = source.getFirstElement("id", fileParts[3], false);

      if(fileParts[2] == '#') {
        fileStr = dom.getContent().toString();
      }
      else {
        fileStr = dom.toString();
      }
    }

    return doT.template(fileStr)(data);
  }

  _M.send = function(str) {
    for(var fn of beforeSendHooks) {
      fn.call(_M, $request, $response);
    }
    $response.send(str);
  }

  _M.status = function(code) {
    $response.status(code);
    return _M;
  }

  _M.html = function(tpl, it) {
    var html = _M.render(tpl, it);

    _M.header('content-type', 'text/html;charset=utf-8');
    _M.send(html);
  }

  _M.respJson = function(data, code, msg) {
    code = code || 200;
    _M.status(code)
      .header('content-type', 'application/json')
      .send(ht.json.JSONUtil.toJsonStr({
        data: Java.asJSONCompatible(data), 
        code: code, 
        msg: msg || '请求成功'
      }));
  };

  _M.respError = function(code, msg) {
    _M.respJson(null, code, msg);
  };

  _M.body = function() {
    return $request.content().as(JavaStr.class);
  }

  _M.reqJson = function() {
    return JSON.parse(_M.body());
  };

  _M.noError = function(msg, fn) {
    return function() {
      try {
        fn.apply(_M, arguments);
      }
      catch(e) {
        _M.send(msg || '');
      }
    }
  };

  _M.header = function(name) {
    if(arguments.length == 1) {
      return $request
        .headers()
        .stream()
        .filter(it => it.name().equalsIgnoreCase(name))
        .findFirst()
        .orElse(null);
    }
    else {
      $response.header(name, arguments[1]);
      return _M;
    }
  };

  _M.type = function(ct) {
    _M.header('content-type', ct);
    return _M;
  };

  _M.query = function(key) {
    return $request.query();
  };

  _M.queryMap = function() {
    var result = {};
    var query = $request.query();
    query.names().forEach(name => result[name] = query.get(name));
    return result;
  }

  _M.request = function() {
    return $request;
  };

  _M.response = function() {
    return $response;
  };

  _M.bind = function(name, fn) {
    _M[name] = function(){
      return fn.apply(_M, arguments);
    };

    return _M;
  }

  return _M;
});
