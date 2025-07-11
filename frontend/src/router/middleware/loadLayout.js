export async function loadLayout(route) {
  let layout = route.meta.layout;
  if (!layout) {
    layout = "default";
  }
  let layoutComponent = await import(`@/layouts/${layout}.vue`);
  route.meta.layoutComponent = layoutComponent.default;

}
