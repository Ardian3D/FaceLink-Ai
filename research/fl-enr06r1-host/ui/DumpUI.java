import com.android.uiautomator.core.Configurator;
import com.android.uiautomator.testrunner.UiAutomatorTestCase;
import android.view.accessibility.AccessibilityNodeInfo;
import android.graphics.Rect;
import org.xmlpull.v1.XmlSerializer;
import android.util.Xml;
import java.io.FileOutputStream;
public class DumpUI extends UiAutomatorTestCase {
    private int count;
    private void node(XmlSerializer x, AccessibilityNodeInfo n, int depth) throws Exception {
        if (n == null || depth > 40 || ++count > 4000) return;
        Rect r = new Rect(); n.getBoundsInScreen(r);
        x.startTag(null,"node");
        x.attribute(null,"text",n.getText()==null?"":n.getText().toString());
        x.attribute(null,"content-desc",n.getContentDescription()==null?"":n.getContentDescription().toString());
        x.attribute(null,"class",n.getClassName()==null?"":n.getClassName().toString());
        x.attribute(null,"clickable",Boolean.toString(n.isClickable()));
        x.attribute(null,"enabled",Boolean.toString(n.isEnabled()));
        x.attribute(null,"bounds","["+r.left+","+r.top+"]["+r.right+","+r.bottom+"]");
        for(int i=0;i<n.getChildCount();i++) { AccessibilityNodeInfo c=n.getChild(i); if(c!=null) { node(x,c,depth+1); c.recycle(); } }
        x.endTag(null,"node");
    }
    public void testDump() throws Exception {
        Configurator.getInstance().setWaitForIdleTimeout(0);
        java.lang.reflect.Method access=getUiDevice().getClass().getDeclaredMethod("getAutomatorBridge");
        access.setAccessible(true); Object bridge=access.invoke(getUiDevice());
        java.lang.reflect.Method root=bridge.getClass().getSuperclass().getDeclaredMethod("getRootInActiveWindow");
        root.setAccessible(true);
        AccessibilityNodeInfo n=null;
        for(int i=0;i<20 && n==null;i++) { Thread.sleep(100); n=(AccessibilityNodeInfo)root.invoke(bridge); }
        if(n==null) throw new IllegalStateException("No accessibility root");
        try(FileOutputStream out=new FileOutputStream("/data/local/tmp/facelink-enr06r1.xml")) {
            XmlSerializer x=Xml.newSerializer(); x.setOutput(out,"UTF-8"); x.startDocument("UTF-8",true);
            x.startTag(null,"hierarchy"); node(x,n,0); x.endTag(null,"hierarchy"); x.endDocument(); x.flush();
        }
        n.recycle();
    }
}
