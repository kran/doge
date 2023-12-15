import org.openjdk.nashorn.api.scripting.NashornScriptEngineFactory;

import javax.script.ScriptException;

public class doge {
    public static void main(String[] args) throws ScriptException {
        var engine = new NashornScriptEngineFactory().getScriptEngine("-scripting", "--language=es6");
        var env = engine.createBindings();
        env.put("$engine", engine);
        env.put("$args", args);
        engine.eval(String.format("load('%s')", args[0]), env);
    }
}