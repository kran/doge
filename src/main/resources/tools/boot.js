const File = java.io.File;
const Charset = java.nio.charset.Charset;
const ht = Packages.cn.hutool;
const htc = ht.core;
const juc = java.util.concurrent;
const _G = $engine.createBindings();
const helidon = Packages.io.helidon;
const JavaStr = java.lang.String;
const ArrayList = java.util.ArrayList;
const Thread = java.lang.Thread;
const TimeZone = java.util.TimeZone;

const genDb = function(c, poolSize) {
  var jdbcUrl = JavaStr.format(c.jdbcUrl, c.h2TcpPort);
  var hc = new com.zaxxer.hikari.HikariConfig();
  hc.setJdbcUrl(jdbcUrl);
  hc.setUsername(c.jdbcUser);
  hc.setPassword(c.jdbcPassword);
  hc.setThreadFactory(Thread.ofVirtual().factory());
  hc.setMaximumPoolSize(poolSize || c.jdbcPoolSize);
  var datasource = new com.zaxxer.hikari.HikariDataSource(hc);

  return org.jooq.impl.DSL.using(datasource, org.jooq.SQLDialect.valueOf(c.sqlDialect));
};

;(function(configFile){
  TimeZone.setDefault(TimeZone.getTimeZone("Asia/Shanghai"));

  { //config
    print(`加载配置文件: ${configFile}`);
    var str = htc.io.FileUtil.readString(new File(configFile), Charset.defaultCharset());
    _G.$config = JSON.parse(str);
  }

  var c = _G.$config;

  { //misc
    _G.$cache = new juc.ConcurrentHashMap();
    _G.$hashids = htc.codec.Hashids.create(c.sessionEncryptKey.toCharArray(), 8);
    _G.$log = org.slf4j.LoggerFactory.getLogger("JSENV");
  }

  { //cron
    var scheduler = new ht.cron.Scheduler();
    scheduler.setThreadExecutor(juc.Executors.newVirtualThreadPerTaskExecutor());
    scheduler.start();
    _G.$scheduler = scheduler;
  }

  { //h2
    var server = new org.h2.tools.Server();
    server.runTool(
      "-baseDir", c.h2BaseDir,
      "-web", "-webAllowOthers", "-webPort", c.h2WebPort, "-webExternalNames", c.externalNames,
      "-tcp", "-tcpAllowOthers", "-tcpPort", c.h2TcpPort
    );
  }

  { //datasource
    _G.$db = genDb(c);
  }

  { //http
    var hh = "io.helidon.webserver.http";
    var webHandler = new (Java.type(`${hh}.Handler`))() {
      handle: function(req, resp) {
        var env = new javax.script.SimpleBindings();
        env.putAll(_G);
        env.put('$request', req);
        env.put('$response', resp);
        $engine.eval("load('scripts/$api.js')", env);
      }
    };

    var staticHandler = helidon.webserver.staticcontent.StaticContentService.create(java.nio.file.Paths.get("public"));

    helidon.webserver.WebServer.builder()
      .routing(r => {
        r.register("/static", Java.to([staticHandler], `${hh}.HttpService[]`));
        r.any("/*", Java.to([webHandler], `${hh}.Handler[]`));
      })
      .address(java.net.InetAddress.getByName("0.0.0.0"))
      .port(c.port)
      .build()
      .start();
  }
})($args[0] || "app.json");

;(function() { //jobs
  var submit = 'schedule(String,String,Runnable)';
  var $dbJob = genDb(_G.$config, 3);
  _G.$vodHit = new org.v2u.ys.service.VodHitManager($dbJob, _G.$cache);


  var isRunning = function(id) {
    return _G.$cache.get("jobs-"+id);
  }

  var setRunning = function(id, yes) {
    _G.$cache.put("jobs-"+id, yes);
  }

  var genJob = function(script) {
    return function() {
      if(!isRunning(script)) {
        try {
          setRunning(script, true);
          var env = new javax.script.SimpleBindings();
          env.putAll(_G);
          env.put("$db", $dbJob);
          $engine.eval(`load('${script}')`, env);
        }
        finally {
          setRunning(script, false);
        }
      }
    }
  }

  for(var script in _G.$config.jobs) {
    _G.$scheduler[submit](script, _G.$config.jobs[script], genJob(script));
  }
})();
