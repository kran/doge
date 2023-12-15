(function(){

  load(`${__DIR__}/$global.js`);

  $ctx = load(`${__DIR__}/lib/context.js`)($request, $response, $config);

  $ctx.bind('session', load(`${__DIR__}/lib/session.js`)($config.sessionEncryptKey));
  $ctx.bind('isMobile', function() {
    var ua = this.header('User-Agent').values() || '';
    return ua.toLowerCase().indexOf('mobile') > -1
  });
  $ctx.bind('cacheControl', function(expires, fn) {
    return function() {
      this.header('Cache-Control', 'max-age='+expires);
      fn.apply(this, arguments);
    }
  });


  try {
    const __vc = $ctx.header('cloudfront-viewer-country');
    const __is_spider = $ctx.header('user-agent').values().indexOf('spider') > -1;
    if(!$config.debug && !__is_spider && (!__vc || __vc.values() != 'CN')) {
      $ctx.status(403).send('403 country forbidden');
    }
    else {
      load($ctx.ensureRouteFile());
    }
  }
  catch(e) {
    if(e instanceof $v.Error) {
      $ctx.respJson(e.errors(), 400, '参数错误');
    }
    if(e instanceof SiteError) {
      $ctx.html('error.html', {error: e});
    }
    else {
      $ctx.status(500);

      if($config.debug) {
        throw e;
      }
      else {
        $ctx.html('error.html', {error: new SiteError(500, e.message)});
      }
    }
  }

})();
