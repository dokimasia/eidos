package f.composite_refs;

import f.composite_refs.dep.Target;
import java.util.Map;
import java.util.Optional;
import java.util.function.Function;

public class Holder {
    Optional<Target> f0;
    Map<String, Target> f1;
    Function<Target, Exception> f2;
}
