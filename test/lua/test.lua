healthStatus = {
    status = "Unknown"
}
if Obj.Status ~= nil then
    local ready = "Ready"
    if statusConditionExists(Obj.Status, ready) then
        healthStatus = {
            status="Progressing"
        }
        if isStatusConditionTrue(Obj.Status, ready) then
            healthStatus = {
                status="Healthy"
            }
        end
    end
end