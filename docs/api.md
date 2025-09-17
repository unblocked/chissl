# API Reference

<div id="redoc-container"></div>
<script src="https://cdn.redoc.ly/redoc/latest/bundles/redoc.standalone.js"></script>
<script>
  // Build an absolute URL for the spec based on the current location, ignoring any <base> tag
  (function() {
    var path = window.location.pathname || '/';
    // If we're at /api or /api/, go up one level to the site root (handles /chissl/api/ on Pages and /api/ locally)
    var basePath = path.replace(/\/api\/?$/, '/');
    var specUrl = new URL('openapi-public.yaml', window.location.origin + basePath).href;
    Redoc.init(specUrl, {
      hideDownloadButton: false,
      pathInMiddlePanel: true,
      expandResponses: '200,201,204',
      theme: { colors: { primary: { main: '#0b5ed7' } } }
    }, document.getElementById('redoc-container'))
  })();
</script>

