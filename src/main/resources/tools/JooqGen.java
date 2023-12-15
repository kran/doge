package tools;

import org.jooq.codegen.DefaultGeneratorStrategy;
import org.jooq.meta.Definition;

public class JooqGeneratorStrategy extends DefaultGeneratorStrategy {
    @Override
    public String getJavaClassName(Definition definition, Mode mode) {
        var name = super.getJavaClassName(definition, mode);
        if(mode.equals(Mode.POJO)) {
            name += "Po";
        }

        return name;
    }
}
